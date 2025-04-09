<template>
    <el-menu  class="el-menu-demo" mode="horizontal" :ellipsis="false"
        background-color="#545c64" text-color="#ffffff" menu-trigger="click" @select="handleSelect">
        <div class="flex-grow" />
        <el-menu-item index="0" :style="{display: userInfo.user_id != 0 ?'none':''}">
            <el-button>登录</el-button>
        </el-menu-item>
        <el-sub-menu index="1" :style="{display: userInfo.user_id == 0 ?'none':''}">
            <template #title>
                <el-avatar :src="userInfo.avatar">  </el-avatar>
            </template>
            <el-menu-item index="1-1">退出</el-menu-item>
        </el-sub-menu>
    </el-menu>
</template>

<script lang="ts" setup>
import { ref,onBeforeMount } from 'vue'
import {getCookie,deleteCookie} from '../utils/utils.ts'

let userInfo = ref<{name:string,avatar:string,user_id:number}>({
    name:"",
    user_id:0,
    avatar:"",
})

onBeforeMount(()=> {
    let access_token = getCookie("sso_0voice_access_token")
    console.log(access_token)
    let list = access_token?.split(".")
    if (list) {
        // atob base64解码
        // btoa base64编码
       userInfo.value = JSON.parse(atob(list[1]))
    }
})

const handleSelect = (key: string, keyPath: string[]) => {
    console.log(key,keyPath)
    switch (key) {
        case '0':
            window.location.href = import.meta.env.VITE_USER_CENTER
            break;
        case '1-1':
            deleteCookie("sso_0voice_access_token")
            window.location.href = window.location.href
            break;
        default:
            break;
    }
}
</script>

<style>
.flex-grow {
    flex-grow: 1;
}
</style>