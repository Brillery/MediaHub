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

interface MenuUserInfo {
    // user_id 来自统一登录 JWT payload，用于判断当前是否展示登录入口。
    user_id: number
    // name/avatar 只用于菜单展示，缺失时保持空值，不能影响页面主流程。
    name: string
    avatar: string
}

let userInfo = ref<MenuUserInfo>({
    name:"",
    user_id:0,
    avatar:"",
})

const configuredUserCenterUrl = import.meta.env.VITE_USER_CENTER

onBeforeMount(()=> {
    const parsedUserInfo = parseUserInfoFromToken(getCookie("sso_0voice_access_token"))
    if (parsedUserInfo) {
       userInfo.value = parsedUserInfo
    }
})

const handleSelect = (key: string) => {
    switch (key) {
        case '0':
            window.location.href = getUserCenterUrl()
            break;
        case '1-1':
            deleteCookie("sso_0voice_access_token")
            window.location.href = window.location.href
            break;
        default:
            break;
    }
}

/**
 * 计算登录中心跳转地址。
 *
 * 独立前端部署优先使用 Vite 注入的 `VITE_USER_CENTER`；后端内置 www
 * 可能由没有生产 env 的机器构建，此时回落到同源 `/login`，避免把线上用户
 * 导到 localhost。具体生产域名仍应由部署环境显式注入。
 */
function getUserCenterUrl(): string {
    if (configuredUserCenterUrl) {
        return configuredUserCenterUrl
    }

    return new URL("/login", window.location.origin).toString()
}

/**
 * 从统一登录 access token 中解析菜单用户信息。
 *
 * 这里只做前端展示态解析，不负责校验签名和权限；真正的鉴权仍由后端基于
 * Authorization header 完成。解析失败时回落到未登录态，避免坏 cookie
 * 让整个首页挂载失败，同时也不在控制台输出 token 内容。
 */
function parseUserInfoFromToken(accessToken: string | null): MenuUserInfo | null {
    const payload = accessToken?.split(".")[1]
    if (!payload) {
        return null
    }

    try {
        const normalizedPayload = payload.replace(/-/g, "+").replace(/_/g, "/")
        const paddedPayload = normalizedPayload.padEnd(Math.ceil(normalizedPayload.length / 4) * 4, "=")
        const payloadBytes = Uint8Array.from(atob(paddedPayload), (char) => char.charCodeAt(0))
        const parsedPayload = JSON.parse(new TextDecoder().decode(payloadBytes)) as Partial<MenuUserInfo>

        // 统一登录返回的 user_id 是菜单登录态的唯一判断依据；缺失或类型异常时按未登录处理。
        if (typeof parsedPayload.user_id !== "number" || parsedPayload.user_id <= 0) {
            return null
        }

        return {
            user_id: parsedPayload.user_id,
            name: parsedPayload.name ?? "",
            avatar: parsedPayload.avatar ?? "",
        }
    } catch {
        return null
    }
}
</script>

<style>
.flex-grow {
    flex-grow: 1;
}
</style>
