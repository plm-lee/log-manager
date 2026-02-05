import React, { useState } from 'react';
import {
  Card,
  Upload,
  Button,
  Input,
  Space,
  message,
  Result,
  Typography,
} from 'antd';
import { InboxOutlined, UploadOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { logApi } from '../api';

const { TextArea } = Input;
const { Dragger } = Upload;

/**
 * 日志上传页面
 * 支持文件选择（.log/.txt）和文本粘贴
 */
const LogUpload = () => {
  const navigate = useNavigate();
  const [tag, setTag] = useState('');
  const [pasteText, setPasteText] = useState('');
  const [uploading, setUploading] = useState(false);
  const [result, setResult] = useState(null);

  const handleFileUpload = (file) => {
    setUploading(true);
    setResult(null);
    const formData = new FormData();
    formData.append('file', file);
    if (tag) formData.append('tag', tag);

    logApi
      .uploadLog(formData)
      .then((res) => {
        const { success, failed, total } = res.data;
        setResult({ success, failed, total });
        message.success(`上传完成：成功 ${success} 条，失败 ${failed} 条`);
      })
      .catch((err) => {
        message.error('上传失败：' + (err.response?.data?.message || err.message));
      })
      .finally(() => setUploading(false));
    return false; // 阻止默认上传行为，我们手动处理
  };

  const handlePasteUpload = () => {
    if (!pasteText.trim()) {
      message.warning('请先粘贴日志内容');
      return;
    }
    setUploading(true);
    setResult(null);
    const formData = new FormData();
    formData.append('logs', pasteText);
    if (tag) formData.append('tag', tag);

    logApi
      .uploadLog(formData)
      .then((res) => {
        const { success, failed, total } = res.data;
        setResult({ success, failed, total });
        message.success(`上传完成：成功 ${success} 条，失败 ${failed} 条`);
      })
      .catch((err) => {
        message.error('上传失败：' + (err.response?.data?.message || err.message));
      })
      .finally(() => setUploading(false));
  };

  const resetForm = () => {
    setTag('');
    setPasteText('');
    setResult(null);
  };

  return (
    <div>
      <Card title="日志上传" style={{ marginBottom: 16 }}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <div>
            <Typography.Text strong>标签（可选）：</Typography.Text>
            <Input
              placeholder="如：manual、test"
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              style={{ width: 200, marginLeft: 8 }}
            />
          </div>

          <div>
            <Typography.Text strong>方式一：上传文件</Typography.Text>
            <Dragger
              accept=".log,.txt"
              maxCount={1}
              beforeUpload={handleFileUpload}
              disabled={uploading}
              style={{ marginTop: 8 }}
            >
              <p className="ant-upload-drag-icon">
                <InboxOutlined style={{ fontSize: 48, color: '#1890ff' }} />
              </p>
              <p className="ant-upload-text">点击或拖拽 .log / .txt 文件到此区域</p>
              <p className="ant-upload-hint">支持单文件上传，每行一条日志，最多 10000 行</p>
            </Dragger>
          </div>

          <div>
            <Typography.Text strong>方式二：粘贴文本</Typography.Text>
            <TextArea
              placeholder="粘贴日志内容，每行一条..."
              value={pasteText}
              onChange={(e) => setPasteText(e.target.value)}
              rows={8}
              style={{ marginTop: 8 }}
            />
            <Button
              type="primary"
              icon={<UploadOutlined />}
              onClick={handlePasteUpload}
              loading={uploading}
              style={{ marginTop: 8 }}
            >
              上传粘贴内容
            </Button>
          </div>
        </Space>
      </Card>

      {result && (
        <Card>
          <Result
            status="success"
            title="上传完成"
            subTitle={`成功 ${result.success} 条，失败 ${result.failed} 条，共 ${result.total} 行`}
            extra={[
              <Button key="view" type="primary" onClick={() => navigate('/logs')}>
                查看日志
              </Button>,
              <Button key="again" onClick={resetForm}>
                继续上传
              </Button>,
            ]}
          />
        </Card>
      )}
    </div>
  );
};

export default LogUpload;
