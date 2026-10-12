import axios from 'axios'
import http from './http'

// 三方子系统「对外接口」调用（协议格式响应，不走 /api 包装与统一拦截器）
const proto = axios.create({ timeout: 15000 })

export const subsystemApi = {
  health: () => proto.get('/health').then((r) => r.data),
  deviceTree: () => proto.get('/protocol/sub/system/api/v1/converter/deviceTree').then((r) => r.data),
  realtime: (pointStr) =>
    proto.get('/protocol/sub/system/api/v1/converter/realtime', { params: { pointStr } }).then((r) => r.data),
  setValue: (pointStr, value) =>
    proto.post(`/api/set/${encodeURIComponent(pointStr)}`, { pointId: pointStr, value }).then((r) => r.data),
}

export const api = {
  // 健康检查（不在 /api 下）
  health: () => axios.get('/healthz', { timeout: 5000 }).then((r) => r.data),

  // ---- 设备模板 ----
  templateTypes: () => http.get('/devices/template-types'),
  templates: () => http.get('/devices/templates'),
  templateDetail: (id) => http.get(`/devices/templates/${id}/detail`),
  templatePreview: (formData) =>
    http.post('/devices/templates/import/preview', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000,
    }),
  templateConfirm: (payload) => http.post('/devices/templates/import/confirm', payload),
  templateDelete: (id) => http.delete(`/devices/templates/${id}`),

  // ---- 设备 ----
  devices: () => http.get('/devices'),
  addDevice: (payload) => http.post('/devices', payload),
  deleteDevice: (id) => http.delete(`/devices/${id}`),
  startDevice: (id) => http.post(`/devices/${id}/start`),
  setDeviceEnabled: (id, enabled) => http.put(`/devices/${id}/enabled`, { enabled }),
  stopDevice: (id) => http.post(`/devices/${id}/stop`),
  devicePoints: (id) => http.get(`/devices/${id}/points`),
  deviceHistory: (id, params) => http.get(`/devices/${id}/history`, { params }),
  deviceParams: (id) => http.get(`/devices/${id}/params`),

  // ---- 设备接入目录与协议参数 ----
  deviceCatalog: (selector) => http.get('/device-catalog', { params: { selector } }),
  protocolTypes: () => http.get('/devices/protocols/types'),
  protocolParams: (protocolType) =>
    http.get('/devices/protocols/params', { params: { protocol_type: protocolType } }),
}
