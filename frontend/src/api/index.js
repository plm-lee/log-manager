import axios from 'axios';

// 创建 axios 实例
const api = axios.create({
  baseURL: process.env.REACT_APP_API_BASE_URL || 'http://localhost:8080/api/v1',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

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
   * 获取所有规则名称列表
   * @returns {Promise} 规则名称列表
   */
  getRuleNames: () => {
    return api.get('/logs/rule-names');
  },
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
   * 接收指标数据（通常由 log-filter-monitor 调用）
   * @param {Object} data - 指标数据
   * @returns {Promise} 响应数据
   */
  receiveMetrics: (data) => {
    return api.post('/metrics', data);
  },
};

export { logApi, metricsApi };
export default api;
