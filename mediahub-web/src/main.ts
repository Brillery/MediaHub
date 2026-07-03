import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { ElButton, ElCarousel, ElCarouselItem } from 'element-plus'
import 'element-plus/theme-chalk/base.css'
import 'element-plus/theme-chalk/el-button.css'
import 'element-plus/theme-chalk/el-carousel.css'
import 'element-plus/theme-chalk/el-carousel-item.css'
import 'element-plus/theme-chalk/el-loading.css'
import 'element-plus/theme-chalk/el-message.css'

// 前台页面目前只用到按钮和轮播组件，不全量注册 Element Plus。
// 这样可以减少首屏 JS/CSS 体积；ElMessage、ElLoading 这类服务 API 继续在调用组件中按需 import。
createApp(App)
  .use(ElButton)
  .use(ElCarousel)
  .use(ElCarouselItem)
  .mount('#app')
