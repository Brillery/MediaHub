<template>
    <div>
      <div style="background-color: white;width:55.375rem;height:2.875rem;padding:0.4375rem;border-radius: 5px">
        <div style="position: relative;width:100%;height:100%">
          <el-button type="success" class="grid-content ep-bg-purple"
            style="position:absolute;left:0;height:2.875rem;width:4.875rem;font-family:Microsoft YaHei;font-size: 1.25rem;"
            @click="handleUpload">上传</el-button>
          <input
            style="width:40.5rem;height:2.75rem;position:absolute;left:5.8rem;border:none;padding:0.0625rem;font-size: 1.5rem;color:#606266;outline:0;"
            id="upload-input" v-model="data.url.value" />
          <el-button type="success" class="grid-content ep-bg-purple"
            style="position:absolute;right:0;height:2.875rem;width:8.0625rem;font-family:Microsoft YaHei;font-size: 1.25rem;"
            @click="handleCopy">复制地址</el-button>
        </div>
      </div>
    </div>
</template>
<script lang="ts" setup>
import {ref} from 'vue' 
import {uploadFile} from '../../api/api'
import {ElMessage,ElLoading} from 'element-plus'
const data = {
    url: ref("") 
} 

class fileUploadRes{
    url:string
    constructor(url:string){
        this.url = url
    }
}

function handleUpload() {
    console.log("handleUpload")
    const input = document.createElement('input')
    input.type = "file"
    input.id = "file-upload"
    input.addEventListener("change", (event) => {
        const files = (event.target as HTMLInputElement).files
        if (files && files.length > 0) {
            const loading = ElLoading.service({
                lock: true,
                text: "文件上传中。。。",
                background: "rgba(0,0,0,0.7)"
            })
            //调用上传接口
            const formData = new FormData();
            formData.append("file", files[0])
            uploadFile<fileUploadRes>({formData:formData}).then(function(res){
                data.url.value = res.data?.url??"" 
            }).catch(function(res){
                console.log(res.message)
            }).finally(function(){
                loading.close()
            })
        }
    })
    input.click()
}
 function handleCopy(){
    navigator.clipboard.writeText(data.url.value).then(function(){
        ElMessage({
            message:"已复制到剪切板",
            type:"success",
        })
    }).catch(function(){
        ElMessage.error( "复制失败，请手动复制文本框内的链接")
    })
 }
</script>