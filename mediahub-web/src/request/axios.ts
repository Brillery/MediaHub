import axios,{AxiosError} from "axios" 
import { ElMessage } from 'element-plus'
import { getCookie } from "../utils/utils.ts";

const service = axios.create({
    baseURL: import.meta.env.VITE_API_BASE_URL 
  });

  // 添加请求拦截器
  service.interceptors.request.use(function (config) {
    // 在发送请求之前做些什么
    let access_token = getCookie("sso_0voice_access_token")
        if (access_token) {
            config.headers.Authorization = "Bearer " + access_token;
        }
    return config;
  }, function (error) {
    // 对请求错误做些什么
    return Promise.reject(error);
  });


// 添加响应拦截器
service.interceptors.response.use(function (response) {
    // 2xx 范围内的状态码都会触发该函数。
    // 对响应数据做点什么
    return response;
  }, function (error) {
    // 超出 2xx 范围的状态码都会触发该函数。
    // 对响应错误做点什么
    const axiosErr = error as AxiosError 
    //    console.log(axiosErr.response?.status)
    if (!axiosErr.response?.status) {
        ElMessage({
            showClose: true,
            message: axiosErr.message,
            type: 'error',
        })
    }else if(axiosErr.response?.status == 500) {
        ElMessage({
            showClose: true,
            message: "服务器内部错误",
            type: 'error',
        })

    }else if(axiosErr.response?.status==504) {
        ElMessage({
            showClose: true,
            message: "网关超时",
            type: 'error',
        })
    }else if(axiosErr.response?.status == 413) {
        ElMessage({
            showClose: true,
            message: "仅支持上传20M以内的图片",
            type: 'error',
        })
    }

    console.log(axiosErr.message)
    console.log(axiosErr.response?.status)

    return Promise.reject(error);
  });

  export default  service