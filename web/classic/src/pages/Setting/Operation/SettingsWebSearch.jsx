/*
Copyright (C) 2023-2026 QuantumNous

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

import React, { useEffect, useState, useRef } from 'react';
import { Button, Col, Form, Row, Spin, Banner } from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const PROVIDER_DUCKDUCKGO = 'duckduckgo';
const PROVIDER_TAVILY = 'tavily';

export default function SettingsWebSearch(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'web_search_setting.provider': PROVIDER_DUCKDUCKGO,
    'web_search_setting.tavily_api_key': '',
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) =>
      API.put('/api/option/', {
        key: item.key,
        value: String(inputs[item.key] ?? ''),
      }),
    );
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length > 1 && res.includes(undefined)) {
          return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key] || inputs[key];
      }
    }
    for (let key of Object.keys(inputs)) {
      if (!(key in currentInputs)) {
        currentInputs[key] = inputs[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current?.setValues(currentInputs);
  }, [props.options]);

  const provider = inputs['web_search_setting.provider'] || PROVIDER_DUCKDUCKGO;

  return (
    <Spin spinning={loading}>
      <Form
        values={inputs}
        getFormApi={(formAPI) => (refForm.current = formAPI)}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('联网搜索设置')}>
          <Banner
            type='info'
            description={t(
              'DuckDuckGo 仅支持关键词类查询（免费、无需配置）。如需通用网页搜索请选择 Tavily 并填写 API Key；所选服务商请求失败时将自动回退到 DuckDuckGo。',
            )}
            style={{ marginBottom: 16 }}
          />
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Select
                field='web_search_setting.provider'
                label={t('搜索服务商')}
                optionList={[
                  {
                    value: PROVIDER_DUCKDUCKGO,
                    label: t('DuckDuckGo（免费，内置）'),
                  },
                  { value: PROVIDER_TAVILY, label: 'Tavily' },
                ]}
                onChange={(value) =>
                  setInputs({ ...inputs, 'web_search_setting.provider': value })
                }
              />
            </Col>
            {provider === PROVIDER_TAVILY && (
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Input
                  field='web_search_setting.tavily_api_key'
                  label={t('Tavily API Key')}
                  placeholder='tvly-...'
                  type='password'
                  extraText={t(
                    '可在 tavily.com 免费获取（免费套餐每月 1,000 次请求）',
                  )}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'web_search_setting.tavily_api_key': value,
                    })
                  }
                />
              </Col>
            )}
          </Row>
          <Button onClick={onSubmit}>{t('保存联网搜索设置')}</Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
