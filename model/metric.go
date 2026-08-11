package model

// MetricSeries identifies one sampled numeric time series. Labels are stored as
// canonical JSON and are addressed through LabelsHash; callers must not query
// labels with database-specific JSON operators.
type MetricSeries struct {
	Id            int64  `json:"id" gorm:"primaryKey"`
	MetricKey     string `json:"metric_key" gorm:"type:varchar(128);uniqueIndex:idx_metric_series_identity,priority:1;index:idx_metric_series_subject,priority:3"`
	SubjectType   string `json:"subject_type" gorm:"type:varchar(64);uniqueIndex:idx_metric_series_identity,priority:2;index:idx_metric_series_subject,priority:1"`
	SubjectId     string `json:"subject_id" gorm:"type:varchar(128);uniqueIndex:idx_metric_series_identity,priority:3;index:idx_metric_series_subject,priority:2"`
	LabelsHash    string `json:"labels_hash" gorm:"type:char(64);uniqueIndex:idx_metric_series_identity,priority:4"`
	Labels        string `json:"labels" gorm:"type:text"`
	Unit          string `json:"unit" gorm:"type:varchar(32)"`
	BucketSeconds int64  `json:"bucket_seconds"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
}

func (MetricSeries) TableName() string {
	return "metric_series"
}

// MetricPoint stores the latest observation for a series inside a source
// bucket. ObservedAtMs orders concurrent or replayed observations in the same
// bucket without relying on response completion order.
type MetricPoint struct {
	Id           int64   `json:"id" gorm:"primaryKey"`
	SeriesId     int64   `json:"series_id" gorm:"uniqueIndex:idx_metric_point_series_bucket,priority:1;index"`
	BucketTs     int64   `json:"bucket_ts" gorm:"bigint;uniqueIndex:idx_metric_point_series_bucket,priority:2;index"`
	ObservedAtMs int64   `json:"observed_at_ms" gorm:"bigint"`
	Value        float64 `json:"value"`
}

func (MetricPoint) TableName() string {
	return "metric_points"
}
