import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Typography,
  message,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  Popconfirm,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import { PlusOutlined, EditOutlined, DeleteOutlined, ArrowLeftOutlined } from '@ant-design/icons';
import { billingApi } from '../api';

const { Title } = Typography;

const matchTypeOptions = [
  { value: 'tag', label: '按标签 (tag)' },
  { value: 'rule_name', label: '按规则名 (rule_name)' },
  { value: 'log_line_contains', label: '按日志内容包含 (log_line)' },
];

/**
 * 计费配置页面
 * 管理计费规则：bill_key、match_type、match_value、unit_price
 */
const BillingConfig = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [configs, setConfigs] = useState([]);
  const [billingProjectTags, setBillingProjectTags] = useState([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [form] = Form.useForm();

  const loadConfigs = async () => {
    setLoading(true);
    try {
      const res = await billingApi.getConfigs();
      setConfigs(res.data.data || []);
    } catch (err) {
      message.error('加载配置失败');
    } finally {
      setLoading(false);
    }
  };

  const loadBillingProjectTags = async () => {
    try {
      const res = await billingApi.getBillingProjectTags();
      setBillingProjectTags(res.data.data || []);
    } catch {
      setBillingProjectTags([]);
    }
  };

  useEffect(() => {
    loadConfigs();
    loadBillingProjectTags();
  }, []);

  const handleAdd = () => {
    setEditingId(null);
    form.resetFields();
    loadBillingProjectTags();
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingId(record.id);
    form.setFieldsValue({
      bill_key: record.bill_key,
      billing_tag: record.billing_tag || '',
      match_type: record.match_type,
      match_value: record.match_value,
      unit_price: record.unit_price,
      description: record.description || '',
    });
    loadBillingProjectTags();
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      await billingApi.deleteConfig(id);
      message.success('删除成功');
      loadConfigs();
    } catch (err) {
      message.error('删除失败');
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      const payload = { ...values };
      if (editingId) {
        await billingApi.updateConfig(editingId, payload);
        message.success('更新成功');
      } else {
        await billingApi.createConfig(payload);
        message.success('新增成功');
      }
      setModalVisible(false);
      loadConfigs();
    } catch (err) {
      if (err.errorFields) return;
      message.error(err.response?.data?.message || '操作失败');
    }
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '计费类型 (bill_key)',
      dataIndex: 'bill_key',
      key: 'bill_key',
    },
    {
      title: '匹配方式',
      dataIndex: 'match_type',
      key: 'match_type',
      width: 140,
      render: (v) => matchTypeOptions.find((o) => o.value === v)?.label || v,
    },
    {
      title: '匹配值',
      dataIndex: 'match_value',
      key: 'match_value',
    },
    {
      title: '计费 Tag',
      dataIndex: 'billing_tag',
      key: 'billing_tag',
      width: 140,
      ellipsis: true,
    },
    {
      title: '单价',
      dataIndex: 'unit_price',
      key: 'unit_price',
      width: 100,
      render: (v) => (v != null ? v.toFixed(4) : '-'),
    },
    {
      title: '备注',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
          <Popconfirm
            title="确定删除此配置？"
            onConfirm={() => handleDelete(record.id)}
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={4} className="lm-page-title">
        计费配置
      </Title>
      <Card>
        <p style={{ marginBottom: 16, color: 'var(--lm-text-secondary)' }}>
          配置计费规则：按标签、规则名或日志内容匹配计费日志，设置单价后可按日统计计费金额。
        </p>
        <Space style={{ marginBottom: 16 }}>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            新增配置
          </Button>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/billing')}>
            返回计费统计
          </Button>
        </Space>
        <Table
          columns={columns}
          dataSource={configs}
          rowKey="id"
          loading={loading}
          pagination={false}
        />
      </Card>

      <Modal
        title={editingId ? '编辑计费配置' : '新增计费配置'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        destroyOnClose
        okText="确定"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="billing_tag"
            label="计费 Tag（必选）"
            extra="请先从已标记为计费项目的标签中选择一个。该规则仅对选中的 tag 生效。若列表为空，请先在「分类管理」中将标签设置到计费项目。"
            rules={[{ required: true, message: '请选择计费 Tag' }]}
          >
            <Select
              placeholder="选择计费项目下的 Tag"
              allowClear
              options={billingProjectTags.map((t) => ({ label: t, value: t }))}
              onChange={(v) => {
                if (v) {
                  form.setFieldsValue({ match_type: 'tag', match_value: v });
                }
              }}
            />
          </Form.Item>
          <Form.Item
            name="bill_key"
            label="计费类型标识 (bill_key)"
            rules={[{ required: true, message: '请输入' }]}
          >
            <Input placeholder="如 submitConfirmOpOrder" />
          </Form.Item>
          <Form.Item
            name="match_type"
            label="匹配方式"
            rules={[{ required: true }]}
          >
            <Select options={matchTypeOptions} placeholder="请选择" />
          </Form.Item>
          <Form.Item
            name="match_value"
            label="匹配值"
            rules={[{ required: true, message: '请输入' }]}
          >
            <Input placeholder="如 submitConfirmOpOrder、mt-api-bill-incr 等" />
          </Form.Item>
          <Form.Item
            name="unit_price"
            label="单价"
            rules={[{ required: true, message: '请输入单价' }]}
          >
            <InputNumber
              min={0}
              step={0.0001}
              precision={4}
              style={{ width: '100%' }}
              placeholder="如 0.01"
            />
          </Form.Item>
          <Form.Item name="description" label="备注">
            <Input.TextArea rows={2} placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default BillingConfig;
