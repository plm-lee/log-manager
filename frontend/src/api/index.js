import axios from 'axios';

const TOKEN_KEY = 'lm_token';

// 创建 axios 实例
// 生产构建（后端托管前端时）使用相对路径；开发时使用 localhost:8888
const api = axios.create({
  baseURL: process.env.REACT_APP_API_BASE_URL || (process.env.NODE_ENV === 'production' ? '/log/manager/api/v1' : 'http://localhost:8888/log/manager/api/v1'),
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截：携带 JWT
api.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY);
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// 响应拦截：401 清除 token 并跳转登录
api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem(TOKEN_KEY);
      if (!window.location.pathname.endsWith('/login') && !window.location.pathname.endsWith('/login/')) {
        window.location.replace(
          process.env.NODE_ENV === 'production' ? '/log/manager/login' : '/login'
        );
      }
    }
    return Promise.reject(err);
  }
);

export const getToken = () => localStorage.getItem(TOKEN_KEY);
export const setToken = (token) => localStorage.setItem(TOKEN_KEY, token);
export const clearToken = () => localStorage.removeItem(TOKEN_KEY);

/**
 * API 接口定义
 */
const logApi = {
  /**
   * 查询日志列表
   * @param {Object} params - 查询参数
   * @param {string} params.tag - 标签筛选
   * @param {string} params.rule_name - 规则名称筛选
   * @param {string} params.keyword - 关键词搜索
   * @param {number} params.start_time - 开始时间戳
   * @param {number} params.end_time - 结束时间戳
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @returns {Promise} 响应数据
   */
  queryLogs: (params) => {
    return api.get('/logs', { params });
  },

  /**
   * 接收日志数据（通常由 log-filter-monitor 调用）
   * @param {Object} data - 日志数据
   * @returns {Promise} 响应数据
   */
  receiveLog: (data) => {
    return api.post('/logs', data);
  },

  /**
   * 获取所有标签列表
   * @returns {Promise} 标签列表
   */
  getTags: () => {
    return api.get('/logs/tags');
  },

  /**
   * 获取标签统计（含日志数量）
   * @returns {Promise} { tags: [{ tag, count }] }
   */
  getTagStats: () => {
    return api.get('/logs/tags/stats');
  },

  /**
   * 获取标签管理列表（含项目信息、日志数）
   * @returns {Promise} { tags: [{ tag, count, project_id, project_name, project_type }] }
   */
  getManagedTags: () => api.get('/logs/tags/managed'),

  /**
   * 设置标签所属项目
   * @param {string} name - tag 名称
   * @param {Object} data - { project_id: number | null }
   */
  setTagProject: (name, data) => api.put(`/logs/tags/${encodeURIComponent(name)}/project`, data),

  /**
   * 获取所有规则名称列表
   * @returns {Promise} 规则名称列表
   */
  getRuleNames: () => {
    return api.get('/logs/rule-names');
  },

  /**
   * 导出日志
   * @param {Object} params - 同 queryLogs，增加 format: 'csv' | 'json'
   * @returns {Promise} blob 响应
   */
  exportLogs: (params) => {
    return api.get('/logs/export', {
      params,
      responseType: 'blob',
    });
  },

  /**
   * 上传日志（文件或粘贴文本）
   * @param {FormData} formData - 包含 file（文件）或 logs（文本），可选 tag
   * @returns {Promise} 响应数据
   */
  uploadLog: (formData) => {
    return api.post('/logs/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000,
    });
  },
};

const tagProjectApi = {
  list: () => api.get('/tag-projects'),
  create: (data) => api.post('/tag-projects', data),
  update: (id, data) => api.put(`/tag-projects/${id}`, data),
  delete: (id) => api.delete(`/tag-projects/${id}`),
};

const metricsApi = {
  /**
   * 查询指标列表
   * @param {Object} params - 查询参数
   * @param {string} params.tag - 标签筛选
   * @param {number} params.start_time - 开始时间戳
   * @param {number} params.end_time - 结束时间戳
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @returns {Promise} 响应数据
   */
  queryMetrics: (params) => {
    return api.get('/metrics', { params });
  },

  /**
   * 查询指标统计数据（用于图表展示）
   * @param {Object} params - 查询参数
   * @param {string} params.tag - 标签筛选
   * @param {number} params.start_time - 开始时间戳
   * @param {number} params.end_time - 结束时间戳
   * @param {string} params.interval - 聚合间隔：1m, 5m, 15m, 1h, 1d（默认1h）
   * @returns {Promise} 响应数据
   */
  queryMetricsStats: (params) => {
    return api.get('/metrics/stats', { params });
  },

  /**
   * 接收指标数据（通常由 log-filter-monitor 调用）
   * @param {Object} data - 指标数据
   * @returns {Promise} 响应数据
   */
  receiveMetrics: (data) => {
    return api.post('/metrics', data);
  },
};

const dashboardApi = {
  getStats: () => api.get('/dashboard/stats'),
};

const authApi = {
  /** 获取认证配置（是否启用登录） */
  getConfig: () => api.get('/auth/config'),
  /** 登录 */
  login: (data) => api.post('/auth/login', data).then((r) => r.data),
  /** 登出 */
  logout: () => api.post('/auth/logout'),
  /** 当前用户（需已登录） */
  me: () => api.get('/auth/me'),
};

const billingApi = {
  /** 获取计费标签列表（从 billing_entries 统计） */
  getTags: () => api.get('/billing/tags'),
  /** 获取计费项目下的 tag 列表（新增配置时需先选择） */
  getBillingProjectTags: () => api.get('/billing/billing-project-tags'),
  /** 获取计费配置列表 */
  getConfigs: () => api.get('/billing/configs'),
  /** 新增计费配置 */
  createConfig: (data) => api.post('/billing/configs', data),
  /** 更新计费配置 */
  updateConfig: (id, data) => api.put(`/billing/configs/${id}`, data),
  /** 删除计费配置 */
  deleteConfig: (id) => api.delete(`/billing/configs/${id}`),
  /** 无匹配规则的计费日志（归属计费项目但未命中规则） */
  getUnmatched: () => api.get('/billing/unmatched'),
  /** 计费统计（参数：start_date, end_date 格式 YYYY-MM-DD，可选 tags 数组） */
  getStats: (params) => {
    const { start_date, end_date, tags } = params;
    let url = `/billing/stats?start_date=${encodeURIComponent(start_date)}&end_date=${encodeURIComponent(end_date)}`;
    if (tags && tags.length) {
      tags.forEach((t) => { url += `&tags=${encodeURIComponent(t)}`; });
    }
    return api.get(url);
  },
  /** 计费按日汇总（主列表，分页） */
  getStatsSummary: (params) => {
    const { start_date, end_date, tags, page = 1, page_size = 20 } = params;
    let url = `/billing/stats?start_date=${encodeURIComponent(start_date)}&end_date=${encodeURIComponent(end_date)}&page=${page}&page_size=${page_size}`;
    if (tags && tags.length) {
      tags.forEach((t) => { url += `&tags=${encodeURIComponent(t)}`; });
    }
    return api.get(url);
  },
  /** 计费日明细（详情弹窗，分页） */
  getStatsDetail: (params) => {
    const { date, tags, page = 1, page_size = 20 } = params;
    let url = `/billing/stats?start_date=${date}&end_date=${date}&date=${encodeURIComponent(date)}&page=${page}&page_size=${page_size}`;
    if (tags && tags.length) {
      tags.forEach((t) => { url += `&tags=${encodeURIComponent(t)}`; });
    }
    return api.get(url);
  },
};

export { logApi, metricsApi, dashboardApi, authApi, billingApi, tagProjectApi };
export default api;
