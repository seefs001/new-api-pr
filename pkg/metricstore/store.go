package metricstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxMetricKeyLength   = 128
	maxSubjectTypeLength = 64
	maxSubjectIDLength   = 128
	maxLabelKeyLength    = 64
	maxLabelValueLength  = 128
	defaultMaxSeries     = 128
)

type Descriptor struct {
	Key               string
	Unit              string
	Bucket            time.Duration
	AllowedSubjects   []string
	RequiredLabels    []string
	OptionalLabels    []string
	MinValue          *float64
	MaxValue          *float64
	MaxSeriesPerScope int
}

type registeredDescriptor struct {
	Descriptor
	allowedSubjects map[string]struct{}
	requiredLabels  map[string]struct{}
	optionalLabels  map[string]struct{}
}

type Subject struct {
	Type string
	ID   string
}

type Measurement struct {
	MetricKey string
	Labels    map[string]string
	Value     float64
}

type Observation struct {
	Subject      Subject
	ObservedAt   time.Time
	Measurements []Measurement
}

type WriteResult struct {
	Inserted int
	Updated  int
	Stale    int
}

type Query struct {
	MetricKey string
	Subject   Subject
	StartTime time.Time
	EndTime   time.Time
	MaxSeries int
}

type Point struct {
	BucketTs     int64
	ObservedAtMs int64
	Value        float64
}

type Series struct {
	MetricKey     string
	Subject       Subject
	Labels        map[string]string
	Unit          string
	BucketSeconds int64
	Points        []Point
}

var descriptorRegistry = struct {
	sync.RWMutex
	values map[string]registeredDescriptor
}{values: map[string]registeredDescriptor{}}

func Register(descriptor Descriptor) error {
	descriptor.Key = strings.TrimSpace(descriptor.Key)
	descriptor.Unit = strings.TrimSpace(descriptor.Unit)
	if descriptor.Key == "" || len(descriptor.Key) > maxMetricKeyLength {
		return errors.New("metric key is required and must not exceed 128 characters")
	}
	if descriptor.Unit == "" || len(descriptor.Unit) > 32 {
		return errors.New("metric unit is required and must not exceed 32 characters")
	}
	if descriptor.Bucket < time.Second || descriptor.Bucket%time.Second != 0 {
		return errors.New("metric bucket must be a whole number of seconds")
	}
	if len(descriptor.AllowedSubjects) == 0 {
		return errors.New("metric must allow at least one subject type")
	}
	if descriptor.MaxSeriesPerScope <= 0 {
		descriptor.MaxSeriesPerScope = defaultMaxSeries
	}
	if descriptor.MinValue != nil && descriptor.MaxValue != nil && *descriptor.MinValue > *descriptor.MaxValue {
		return errors.New("metric minimum value exceeds maximum value")
	}

	registered := registeredDescriptor{
		Descriptor:      descriptor,
		allowedSubjects: map[string]struct{}{},
		requiredLabels:  map[string]struct{}{},
		optionalLabels:  map[string]struct{}{},
	}
	for _, subjectType := range descriptor.AllowedSubjects {
		subjectType = strings.TrimSpace(subjectType)
		if subjectType == "" || len(subjectType) > maxSubjectTypeLength {
			return errors.New("invalid allowed subject type")
		}
		registered.allowedSubjects[subjectType] = struct{}{}
	}
	for _, key := range descriptor.RequiredLabels {
		if err := registerLabelKey(registered.requiredLabels, registered.optionalLabels, key); err != nil {
			return err
		}
	}
	for _, key := range descriptor.OptionalLabels {
		if err := registerLabelKey(registered.optionalLabels, registered.requiredLabels, key); err != nil {
			return err
		}
	}

	descriptorRegistry.Lock()
	defer descriptorRegistry.Unlock()
	if existing, ok := descriptorRegistry.values[descriptor.Key]; ok {
		if descriptorsEqual(existing, registered) {
			return nil
		}
		return fmt.Errorf("metric %q is already registered with a different definition", descriptor.Key)
	}
	descriptorRegistry.values[descriptor.Key] = registered
	return nil
}

func MustRegister(descriptor Descriptor) {
	if err := Register(descriptor); err != nil {
		panic(err)
	}
}

func RecordObservation(ctx context.Context, observation Observation) (WriteResult, error) {
	prepared, err := prepareObservation(observation)
	if err != nil {
		return WriteResult{}, err
	}

	result := WriteResult{}
	err = model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, measurement := range prepared {
			series, err := resolveSeries(tx, observation.Subject, measurement)
			if err != nil {
				return err
			}
			writeState, err := writePoint(tx, series.Id, measurement.bucketTs, observation.ObservedAt.UnixMilli(), measurement.Value)
			if err != nil {
				return err
			}
			switch writeState {
			case pointInserted:
				result.Inserted++
			case pointUpdated:
				result.Updated++
			case pointStale:
				result.Stale++
			}
		}
		return nil
	})
	return result, err
}

func QuerySeries(ctx context.Context, query Query) ([]Series, error) {
	descriptor, err := getDescriptor(query.MetricKey)
	if err != nil {
		return nil, err
	}
	if err := validateSubject(descriptor, query.Subject); err != nil {
		return nil, err
	}
	if query.StartTime.IsZero() || query.EndTime.IsZero() || !query.StartTime.Before(query.EndTime) {
		return nil, errors.New("metric query requires a valid time range")
	}
	if query.MaxSeries <= 0 {
		query.MaxSeries = defaultMaxSeries
	}
	if query.MaxSeries > defaultMaxSeries {
		query.MaxSeries = defaultMaxSeries
	}

	var rows []model.MetricSeries
	err = model.DB.WithContext(ctx).
		Where("metric_key = ? AND subject_type = ? AND subject_id = ?", query.MetricKey, query.Subject.Type, query.Subject.ID).
		Order("id ASC").
		Limit(query.MaxSeries + 1).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) > query.MaxSeries {
		return nil, fmt.Errorf("metric query exceeds the maximum of %d series", query.MaxSeries)
	}
	if len(rows) == 0 {
		return []Series{}, nil
	}

	ids := make([]int64, 0, len(rows))
	seriesByID := make(map[int64]int, len(rows))
	result := make([]Series, 0, len(rows))
	for _, row := range rows {
		var labels map[string]string
		if err := common.UnmarshalJsonStr(row.Labels, &labels); err != nil {
			return nil, fmt.Errorf("decode metric series labels: %w", err)
		}
		seriesByID[row.Id] = len(result)
		ids = append(ids, row.Id)
		result = append(result, Series{
			MetricKey:     row.MetricKey,
			Subject:       query.Subject,
			Labels:        labels,
			Unit:          row.Unit,
			BucketSeconds: row.BucketSeconds,
			Points:        []Point{},
		})
	}

	var points []model.MetricPoint
	err = model.DB.WithContext(ctx).
		Where("series_id IN ? AND bucket_ts >= ? AND bucket_ts <= ?", ids, query.StartTime.Unix(), query.EndTime.Unix()).
		Order("series_id ASC, bucket_ts ASC").
		Find(&points).Error
	if err != nil {
		return nil, err
	}
	for _, point := range points {
		index, ok := seriesByID[point.SeriesId]
		if !ok {
			continue
		}
		result[index].Points = append(result[index].Points, Point{
			BucketTs:     point.BucketTs,
			ObservedAtMs: point.ObservedAtMs,
			Value:        point.Value,
		})
	}
	return result, nil
}

func Prune(ctx context.Context, metricKey string, before time.Time) (int64, error) {
	if _, err := getDescriptor(metricKey); err != nil {
		return 0, err
	}
	if before.IsZero() {
		return 0, errors.New("metric prune cutoff is required")
	}
	var seriesIDs []int64
	if err := model.DB.WithContext(ctx).Model(&model.MetricSeries{}).
		Where("metric_key = ?", metricKey).
		Pluck("id", &seriesIDs).Error; err != nil {
		return 0, err
	}
	if len(seriesIDs) == 0 {
		return 0, nil
	}
	const deleteBatchSize = 2000
	var deleted int64
	for {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		var pointIDs []int64
		if err := model.DB.WithContext(ctx).Model(&model.MetricPoint{}).
			Where("series_id IN ? AND bucket_ts < ?", seriesIDs, before.Unix()).
			Order("id ASC").
			Limit(deleteBatchSize).
			Pluck("id", &pointIDs).Error; err != nil {
			return deleted, err
		}
		if len(pointIDs) == 0 {
			return deleted, nil
		}
		result := model.DB.WithContext(ctx).Where("id IN ?", pointIDs).Delete(&model.MetricPoint{})
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
		if result.RowsAffected == 0 {
			return deleted, errors.New("metric point cleanup made no progress")
		}
	}
}

type preparedMeasurement struct {
	Measurement
	descriptor registeredDescriptor
	labelsJSON string
	labelsHash string
	bucketTs   int64
}

func prepareObservation(observation Observation) ([]preparedMeasurement, error) {
	if observation.ObservedAt.IsZero() {
		return nil, errors.New("metric observation time is required")
	}
	if len(observation.Measurements) == 0 {
		return nil, errors.New("metric observation has no measurements")
	}

	prepared := make([]preparedMeasurement, 0, len(observation.Measurements))
	identities := make(map[string]struct{}, len(observation.Measurements))
	for _, measurement := range observation.Measurements {
		descriptor, err := getDescriptor(measurement.MetricKey)
		if err != nil {
			return nil, err
		}
		if err := validateSubject(descriptor, observation.Subject); err != nil {
			return nil, err
		}
		if err := validateMeasurement(descriptor, measurement); err != nil {
			return nil, err
		}
		labelsJSON, labelsHash, err := canonicalLabels(measurement.Labels)
		if err != nil {
			return nil, err
		}
		identity := measurement.MetricKey + "\x00" + labelsHash
		if _, exists := identities[identity]; exists {
			return nil, fmt.Errorf("duplicate metric measurement for %q", measurement.MetricKey)
		}
		identities[identity] = struct{}{}

		bucketSeconds := int64(descriptor.Bucket / time.Second)
		observedSeconds := observation.ObservedAt.Unix()
		prepared = append(prepared, preparedMeasurement{
			Measurement: measurement,
			descriptor:  descriptor,
			labelsJSON:  labelsJSON,
			labelsHash:  labelsHash,
			bucketTs:    observedSeconds - observedSeconds%bucketSeconds,
		})
	}
	return prepared, nil
}

func resolveSeries(tx *gorm.DB, subject Subject, measurement preparedMeasurement) (*model.MetricSeries, error) {
	where := "metric_key = ? AND subject_type = ? AND subject_id = ? AND labels_hash = ?"
	args := []any{measurement.MetricKey, subject.Type, subject.ID, measurement.labelsHash}
	var series model.MetricSeries
	err := tx.Where(where, args...).First(&series).Error
	if err == nil {
		return validateStoredSeries(&series, measurement)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var count int64
	if err := tx.Model(&model.MetricSeries{}).
		Where("metric_key = ? AND subject_type = ? AND subject_id = ?", measurement.MetricKey, subject.Type, subject.ID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count >= int64(measurement.descriptor.MaxSeriesPerScope) {
		return nil, fmt.Errorf("metric %q exceeds the maximum of %d series for subject", measurement.MetricKey, measurement.descriptor.MaxSeriesPerScope)
	}

	candidate := model.MetricSeries{
		MetricKey:     measurement.MetricKey,
		SubjectType:   subject.Type,
		SubjectId:     subject.ID,
		LabelsHash:    measurement.labelsHash,
		Labels:        measurement.labelsJSON,
		Unit:          measurement.descriptor.Unit,
		BucketSeconds: int64(measurement.descriptor.Bucket / time.Second),
		CreatedAt:     time.Now().Unix(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return nil, err
	}
	if err := tx.Where(where, args...).First(&series).Error; err != nil {
		return nil, err
	}
	return validateStoredSeries(&series, measurement)
}

func validateStoredSeries(series *model.MetricSeries, measurement preparedMeasurement) (*model.MetricSeries, error) {
	if series.Labels != measurement.labelsJSON {
		return nil, errors.New("metric labels hash collision detected")
	}
	bucketSeconds := int64(measurement.descriptor.Bucket / time.Second)
	if series.Unit != measurement.descriptor.Unit || series.BucketSeconds != bucketSeconds {
		return nil, fmt.Errorf("stored metric series %d conflicts with registered definition", series.Id)
	}
	return series, nil
}

type pointWriteState int

const (
	pointInserted pointWriteState = iota
	pointUpdated
	pointStale
)

func writePoint(tx *gorm.DB, seriesID int64, bucketTs int64, observedAtMs int64, value float64) (pointWriteState, error) {
	updated := tx.Model(&model.MetricPoint{}).
		Where("series_id = ? AND bucket_ts = ? AND observed_at_ms < ?", seriesID, bucketTs, observedAtMs).
		Updates(map[string]any{"observed_at_ms": observedAtMs, "value": value})
	if updated.Error != nil {
		return pointStale, updated.Error
	}
	if updated.RowsAffected > 0 {
		return pointUpdated, nil
	}

	var existing model.MetricPoint
	err := tx.Where("series_id = ? AND bucket_ts = ?", seriesID, bucketTs).First(&existing).Error
	if err == nil {
		return pointStale, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return pointStale, err
	}

	point := model.MetricPoint{SeriesId: seriesID, BucketTs: bucketTs, ObservedAtMs: observedAtMs, Value: value}
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&point)
	if created.Error != nil {
		return pointStale, created.Error
	}
	if created.RowsAffected > 0 {
		return pointInserted, nil
	}

	updated = tx.Model(&model.MetricPoint{}).
		Where("series_id = ? AND bucket_ts = ? AND observed_at_ms < ?", seriesID, bucketTs, observedAtMs).
		Updates(map[string]any{"observed_at_ms": observedAtMs, "value": value})
	if updated.Error != nil {
		return pointStale, updated.Error
	}
	if updated.RowsAffected > 0 {
		return pointUpdated, nil
	}
	return pointStale, nil
}

func getDescriptor(metricKey string) (registeredDescriptor, error) {
	descriptorRegistry.RLock()
	defer descriptorRegistry.RUnlock()
	descriptor, ok := descriptorRegistry.values[metricKey]
	if !ok {
		return registeredDescriptor{}, fmt.Errorf("metric %q is not registered", metricKey)
	}
	return descriptor, nil
}

func validateSubject(descriptor registeredDescriptor, subject Subject) error {
	if subject.Type == "" || len(subject.Type) > maxSubjectTypeLength {
		return errors.New("invalid metric subject type")
	}
	if subject.ID == "" || len(subject.ID) > maxSubjectIDLength {
		return errors.New("invalid metric subject id")
	}
	if _, ok := descriptor.allowedSubjects[subject.Type]; !ok {
		return fmt.Errorf("metric %q does not allow subject type %q", descriptor.Key, subject.Type)
	}
	return nil
}

func validateMeasurement(descriptor registeredDescriptor, measurement Measurement) error {
	if math.IsNaN(measurement.Value) || math.IsInf(measurement.Value, 0) {
		return fmt.Errorf("metric %q value must be finite", descriptor.Key)
	}
	if descriptor.MinValue != nil && measurement.Value < *descriptor.MinValue {
		return fmt.Errorf("metric %q value is below its minimum", descriptor.Key)
	}
	if descriptor.MaxValue != nil && measurement.Value > *descriptor.MaxValue {
		return fmt.Errorf("metric %q value exceeds its maximum", descriptor.Key)
	}
	if len(measurement.Labels) != len(descriptor.requiredLabels)+countPresentOptionalLabels(descriptor, measurement.Labels) {
		return fmt.Errorf("metric %q has unsupported labels", descriptor.Key)
	}
	for key := range descriptor.requiredLabels {
		if _, ok := measurement.Labels[key]; !ok {
			return fmt.Errorf("metric %q requires label %q", descriptor.Key, key)
		}
	}
	for key, value := range measurement.Labels {
		if len(key) == 0 || len(key) > maxLabelKeyLength || len(value) > maxLabelValueLength {
			return fmt.Errorf("metric %q has an invalid label", descriptor.Key)
		}
		if _, required := descriptor.requiredLabels[key]; required {
			continue
		}
		if _, optional := descriptor.optionalLabels[key]; !optional {
			return fmt.Errorf("metric %q has unsupported label %q", descriptor.Key, key)
		}
	}
	return nil
}

func countPresentOptionalLabels(descriptor registeredDescriptor, labels map[string]string) int {
	count := 0
	for key := range descriptor.optionalLabels {
		if _, ok := labels[key]; ok {
			count++
		}
	}
	return count
}

func canonicalLabels(labels map[string]string) (string, string, error) {
	if labels == nil {
		labels = map[string]string{}
	}
	data, err := common.Marshal(labels)
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(data)
	return string(data), hex.EncodeToString(digest[:]), nil
}

func registerLabelKey(target map[string]struct{}, other map[string]struct{}, key string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > maxLabelKeyLength {
		return errors.New("invalid metric label key")
	}
	if _, exists := target[key]; exists {
		return fmt.Errorf("duplicate metric label key %q", key)
	}
	if _, exists := other[key]; exists {
		return fmt.Errorf("metric label key %q is both required and optional", key)
	}
	target[key] = struct{}{}
	return nil
}

func descriptorsEqual(left registeredDescriptor, right registeredDescriptor) bool {
	if left.Key != right.Key || left.Unit != right.Unit || left.Bucket != right.Bucket || left.MaxSeriesPerScope != right.MaxSeriesPerScope {
		return false
	}
	if !floatPointersEqual(left.MinValue, right.MinValue) || !floatPointersEqual(left.MaxValue, right.MaxValue) {
		return false
	}
	return stringSetsEqual(left.allowedSubjects, right.allowedSubjects) &&
		stringSetsEqual(left.requiredLabels, right.requiredLabels) &&
		stringSetsEqual(left.optionalLabels, right.optionalLabels)
}

func floatPointersEqual(left *float64, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func stringSetsEqual(left map[string]struct{}, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	keys := make([]string, 0, len(left))
	for key := range left {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := right[key]; !ok {
			return false
		}
	}
	return true
}
