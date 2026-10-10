import axios from 'axios'
import { ElMessage } from 'element-plus'

// 统一 API 客户端：本项目后端不带鉴权，直接请求 /api。
const http = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

http.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const data = error?.response?.data
    const message = data?.error || data?.message || error.message || '请求失败'
    ElMessage.error(message)
    return Promise.reject(error)
  },
)

export default http
