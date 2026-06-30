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

import React, { useRef, useState } from 'react';
import {
  Banner,
  Button,
  Col,
  Form,
  Modal,
  Row,
  Space,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { IconDownload, IconUpload } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

const { Text } = Typography;

function filenameFromDisposition(header) {
  if (!header) return '';
  const match = /filename="?([^";]+)"?/i.exec(header);
  return match?.[1] ?? '';
}

function saveBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export default function SettingsBackup() {
  const { t } = useTranslation();
  const fileInputRef = useRef(null);
  const [selectedFile, setSelectedFile] = useState(null);
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);

  async function onExportBackup() {
    try {
      setExporting(true);
      const response = await API.get('/api/backup/export', {
        responseType: 'blob',
        disableDuplicate: true,
        skipErrorHandler: true,
      });
      const filename =
        filenameFromDisposition(response.headers['content-disposition']) ||
        `new-api-backup-${Date.now()}.json`;
      saveBlob(response.data, filename);
      showSuccess(t('备份已导出'));
    } catch (error) {
      showError(error?.response?.data?.message || t('备份导出失败'));
    } finally {
      setExporting(false);
    }
  }

  async function importBackup() {
    if (!selectedFile) {
      showError(t('请先选择备份文件'));
      return;
    }

    try {
      setImporting(true);
      const formData = new FormData();
      formData.append('file', selectedFile);
      formData.append('confirm', 'IMPORT');
      const response = await API.post('/api/backup/import', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        skipErrorHandler: true,
      });
      const { success, message, data } = response.data;
      if (!success) {
        showError(message || t('备份导入失败'));
        return;
      }
      showSuccess(
        t('备份已导入：{{tables}} 张表，共 {{rows}} 行', {
          tables: data?.imported_tables ?? 0,
          rows: data?.imported_rows ?? 0,
        }),
      );
      setSelectedFile(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
    } catch (error) {
      showError(error?.response?.data?.message || t('备份导入失败'));
    } finally {
      setImporting(false);
    }
  }

  function onImportBackup() {
    if (!selectedFile) {
      showError(t('请先选择备份文件'));
      return;
    }

    Modal.confirm({
      title: t('确认导入备份？'),
      content: (
        <div style={{ lineHeight: 1.8 }}>
          <p>
            <Text>{t('将导入文件')}：</Text>
            <Text strong>{selectedFile.name}</Text>
          </p>
          <Text type='danger'>
            {t(
              '导入备份会替换当前数据库记录，此操作不可恢复，请提前确认已导出当前数据。',
            )}
          </Text>
        </div>
      ),
      okText: t('确认导入'),
      cancelText: t('取消'),
      okType: 'danger',
      onOk: importBackup,
    });
  }

  return (
    <Spin spinning={exporting || importing}>
      <Form style={{ marginBottom: 15 }}>
        <Form.Section text={t('备份与恢复')}>
          <Banner
            type='warning'
            fullMode={false}
            title={t('备份包含敏感数据')}
            description={t(
              '导出的文件包含用户、令牌、渠道、密钥、计费记录和系统设置，请妥善保存。',
            )}
            style={{ marginBottom: 16 }}
          />
          <Row gutter={16}>
            <Col xs={24} sm={12} md={12} lg={12} xl={12}>
              <Space vertical align='start' style={{ width: '100%' }}>
                <Text strong>{t('导出备份')}</Text>
                <Text type='tertiary'>
                  {t('下载当前主数据库的 JSON 备份文件。')}
                </Text>
                <Button
                  icon={<IconDownload />}
                  theme='solid'
                  loading={exporting}
                  onClick={onExportBackup}
                >
                  {t('导出备份')}
                </Button>
              </Space>
            </Col>
            <Col xs={24} sm={12} md={12} lg={12} xl={12}>
              <Space vertical align='start' style={{ width: '100%' }}>
                <Text strong>{t('导入备份')}</Text>
                <Text type='tertiary'>
                  {t('恢复备份文件并替换当前数据库数据。')}
                </Text>
                <input
                  ref={fileInputRef}
                  type='file'
                  accept='application/json,.json'
                  disabled={importing}
                  onChange={(event) =>
                    setSelectedFile(event.target.files?.[0] ?? null)
                  }
                />
                {selectedFile && (
                  <Text type='tertiary'>
                    {t('已选择')}：{selectedFile.name}
                  </Text>
                )}
                <Button
                  icon={<IconUpload />}
                  theme='solid'
                  type='danger'
                  loading={importing}
                  disabled={!selectedFile}
                  onClick={onImportBackup}
                >
                  {t('导入备份')}
                </Button>
              </Space>
            </Col>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
