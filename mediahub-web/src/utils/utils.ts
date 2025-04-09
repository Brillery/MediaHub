export function getUrlParameter(name:string) {
    name = name.replace(/[[]/, '\\[').replace(/[\]]/, '\\]');
    var regex = new RegExp('[\\?&]' + name + '=([^&#]*)');
    var results = regex.exec(window.location.search);
    return results === null ? '' : decodeURIComponent(results[1].replace(/\+/g, ' '));
  }

  export function getCookie(key:string) {
    const cookies = document.cookie.split(';');
    for (let i = 0; i < cookies.length; i++) {
      const cookie = cookies[i].trim();
      if (cookie.startsWith(`${key}=`)) {
        return cookie.substring(key.length + 1);
      }
    }
    return null;
  }

  export function getDateStr(timestamp:number){
    const date = new Date(timestamp * 1000);
    const year = date.getFullYear();
    const month = ("0" + (date.getMonth() + 1)).slice(-2); // 月份从 0 开始，需要加 1，并保证两位数格式
    const day = ("0" + date.getDate()).slice(-2); // 保证日期为两位数格式
    const hours = ("0" + date.getHours()).slice(-2); // 保证小时为两位数格式
    const minutes = ("0" + date.getMinutes()).slice(-2); // 保证分钟为两位数格式
    const seconds = ("0" + date.getSeconds()).slice(-2); // 保证秒钟为两位数格式
    
    // 格式化后的时间字符串
   let formattedTime = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    return formattedTime 
    
  }

  export function setCookie(name: string, value: string, days: number) {
    const date = new Date();
    date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
    
    const expires = "expires=" + date.toUTCString();
    document.cookie = name + "=" + value + ";" + expires + ";path=/";
  }
  export function deleteCookie(name:string) {
    document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
  } 