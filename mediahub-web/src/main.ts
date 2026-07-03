import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { ElAvatar, ElButton, ElCarousel, ElCarouselItem, ElMenu, ElMenuItem, ElSubMenu } from 'element-plus'
import 'element-plus/theme-chalk/base.css'
import 'element-plus/theme-chalk/el-avatar.css'
import 'element-plus/theme-chalk/el-button.css'
import 'element-plus/theme-chalk/el-carousel.css'
import 'element-plus/theme-chalk/el-carousel-item.css'
import 'element-plus/theme-chalk/el-loading.css'
import 'element-plus/theme-chalk/el-menu.css'
import 'element-plus/theme-chalk/el-menu-item.css'
import 'element-plus/theme-chalk/el-message.css'
import 'element-plus/theme-chalk/el-sub-menu.css'

// 前台页面目前只用到菜单、头像、按钮和轮播组件，不全量注册 Element Plus。
// 这样可以减少首屏 JS/CSS 体积；ElMessage、ElLoading 这类服务 API 继续在调用组件中按需 import。
createApp(App)
  .use(ElAvatar)
  .use(ElButton)
  .use(ElCarousel)
  .use(ElCarouselItem)
  .use(ElMenu)
  .use(ElMenuItem)
  .use(ElSubMenu)
  .mount('#app')
