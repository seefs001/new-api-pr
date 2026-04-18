/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { Button, Dropdown } from '@douyinfe/semi-ui';
import { Languages } from 'lucide-react';

const languageOptions = [
  { key: 'zh-CN', label: '简体中文' },
  { key: 'zh-TW', label: '繁體中文' },
  { key: 'en', label: 'English' },
  { key: 'fr', label: 'Français' },
  { key: 'ja', label: '日本語' },
  { key: 'ru', label: 'Русский' },
  { key: 'vi', label: 'Tiếng Việt' },
];

const LanguageSelector = ({ currentLang, onLanguageChange, t }) => {
  return (
    <Dropdown
      position='bottomRight'
      render={
        <Dropdown.Menu className='header-dropdown-menu'>
          {languageOptions.map((option) => (
            <Dropdown.Item
              key={option.key}
              onClick={() => onLanguageChange(option.key)}
              className={`header-dropdown-item ${currentLang === option.key ? '!bg-semi-color-primary-light-default !font-semibold' : ''}`}
            >
              {option.label}
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      }
    >
      <Button
        icon={<Languages size={18} />}
        aria-label={t('common.changeLanguage')}
        theme='borderless'
        type='tertiary'
        className='header-action-button'
      />
    </Dropdown>
  );
};

export default LanguageSelector;
