import {get,post} from '../request/request'

export function uploadFile<T=any>(params:{formData:FormData}){
    const path = "/v1/file/upload"
    return post<T>({
        url:path,
        data:params.formData,
    })
}

export function home<T=any>() {
    const path = "/v1/home"
    return get<T>({url:path})
}