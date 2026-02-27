import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Tag,
  Typography,
  message,
  Space,
  Button,
  Select,
  Modal,
  Form,
  Input,
  Popconfirm,
  Tabs,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import { logApi, tagProjectApi } from '../api';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';

const { Title } = Typography;

/**
 * 分类管理页面
 * 展示标签及对应日志数量，支持设置所属项目、大项目管理
 */
const Classification = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [tags, setTags] = useState([]);
  const [projects, setProjects] = useState([]);
  const [projectModalVisible, setProjectModalVisible] = useState(false);
  const [editingProject, setEditingProject] = useState(null);
  const [projectForm] = Form.useForm();

  const loadData = async () => {
    setLoading(true);
    try {
      const [tagsRes, projectsRes] = await Promise.all([
        logApi.getManagedTags(),
        tagProjectApi.list(),
      ]);
      setTags(tagsRes.data.tags || []);
      setProjects(projectsRes.data.data || []);
    } catch (err) {
      message.error('加载数据失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const billingProject = projects.find((p) => p.type === 'billing');
  const normalProjects = projects.filter((p) => p.type !== 'billing');
  const projectOptions = [
    ...(billingProject ? [{ label: `${billingProject.name}（计费）`, value: billingProject.id }] : []),
    ...normalProjects.map((p) => ({ label: p.name, value: p.id })),
    { label: '无', value: null },
  ];

  const handleSetProject = async (tagName, projectId) => {
    try {
      await logApi.setTagProject(tagName, { project_id: projectId });
      message.success('设置成功');
      loadData();
    } catch (err) {
      message.error('设置失败');
    }
  };

  const handleAddProject = () => {
    setEditingProject(null);
    projectForm.resetFields();
    setProjectModalVisible(true);
  };

  const handleEditProject = (record) => {
    setEditingProject(record);
    projectForm.setFieldsValue({ name: record.name, description: record.description || '' });
    setProjectModalVisible(true);
  };

  const handleDeleteProject = async (id) => {
    try {
      await tagProjectApi.delete(id);
      message.success('删除成功');
      loadData();
    } catch (err) {
      message.error(err.response?.data?.error || '删除失败');
    }
  };

  const handleProjectSubmit = async () => {
    try {
      const values = await projectForm.validateFields();
      if (editingProject) {
        await tagProjectApi.update(editingProject.id, values);
        message.success('更新成功');
      } else {
        await tagProjectApi.create(values);
        message.success('创建成功');
      }
      setProjectModalVisible(false);
      loadData();
    } catch (err) {
      if (err.errorFields) return;
      message.error('操作失败');
    }
  };

  const tagColumns = [
    {
      title: '标签',
      dataIndex: 'tag',
      key: 'tag',
      render: (tag, record) => (
        <Space>
          <Tag color="blue">{tag}</Tag>
          {record.project_type === 'billing' && (
            <Tag color="orange">计费</Tag>
          )}
        </Space>
      ),
    },
    {
      title: '日志数量',
      dataIndex: 'count',
      key: 'count',
      width: 100,
    },
    {
      title: '所属项目',
      key: 'project',
      width: 200,
      render: (_, record) => (
        <Select
          size="small"
          style={{ width: 160 }}
          placeholder="设置项目"
          value={record.project_id}
          allowClear
          options={projectOptions}
          onChange={(v) => handleSetProject(record.tag, v)}
        />
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_, record) => (
        <a onClick={() => navigate(`/logs?tag=${encodeURIComponent(record.tag)}`)}>
          查看日志
        </a>
      ),
    },
  ];

  const projectColumns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: '项目名称', dataIndex: 'name', key: 'name' },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 80,
      render: (v) => (v === 'billing' ? '计费项目' : '普通项目'),
    },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_, record) =>
        record.type === 'billing' ? (
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEditProject(record)}>
            编辑
          </Button>
        ) : (
          <Space>
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEditProject(record)}>
              编辑
            </Button>
            <Popconfirm
              title="确定删除此项目？"
              onConfirm={() => handleDeleteProject(record.id)}
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
        分类管理
      </Title>
      <Card>
        <p style={{ marginBottom: 16, color: 'var(--lm-text-secondary)' }}>
          按标签区分不同项目上报的日志。可将多个标签聚合到大项目中；归属「计费项目」的标签视为计费类型。计费配置新增时需先从计费项目中选择标签。
        </p>
        <Tabs
          defaultActiveKey="tags"
          items={[
            {
              key: 'tags',
              label: '标签',
              children: (
                <Table
                  columns={tagColumns}
                  dataSource={tags}
                  rowKey="tag"
                  loading={loading}
                  pagination={false}
                />
              ),
            },
            {
              key: 'projects',
              label: '大项目',
              children: (
                <>
                  <Space style={{ marginBottom: 16 }}>
                    <Button type="primary" icon={<PlusOutlined />} onClick={handleAddProject}>
                      新增项目
                    </Button>
                  </Space>
                  <Table
                    columns={projectColumns}
                    dataSource={projects}
                    rowKey="id"
                    loading={loading}
                    pagination={false}
                  />
                </>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title={editingProject ? '编辑项目' : '新增项目'}
        open={projectModalVisible}
        onOk={handleProjectSubmit}
        onCancel={() => setProjectModalVisible(false)}
        okText="确定"
        cancelText="取消"
      >
        <Form form={projectForm} layout="vertical">
          <Form.Item name="name" label="项目名称" rules={[{ required: true }]}>
            <Input placeholder="如：计费项目、业务A" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Classification;
