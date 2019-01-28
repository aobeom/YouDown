# YouDown

## 简介
一个基于 golang 的 y2b 视频下载接口，前端使用 React

## 主要功能
自动分析视频最高质量，并下载合并。

## 配置说明

```golang
{
    "listen_addr": "监听地址:端口",
    "redis_addr": "Redis地址:端口",
    "save_path": "视频本地保存的路径",
    "www_path": "web访问的路径"
}
```

## 外部工具

[ffmpeg](http://ffmpeg.org/download.html)  
[redis](https://redis.io/)
