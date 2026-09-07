# 架构与部署边界

SaiAdmin / Art Design Pro 前端 → Go HTTP API → 持久化状态与任务队列 → 内置 Worker → Docker 沙箱。

平台单进程持有状态锁，更新使用临时文件、fsync 和原子重命名。任务领取后先持久化 running，再启动容器。进程重启将遗留 running 标记失败，避免重复第三方副作用。queued 任务继续处理。

容器使用 nobody、只读根文件系统、全部 capability 移除、no-new-privileges、PID/内存限制及 CPU 权重。输入、代码与代理 socket 目录只读，临时目录有容量上限，结果目录使用 64 MB tmpfs。执行结束后收集文件、删除执行容器。不会将 Docker socket 或数据库状态挂载给工具。

出站网络：执行容器始终 `--network none`；socat 将容器 loopback 上的 HTTP 代理端口转发到单任务 Unix socket。Go 代理校验域名白名单及 DNS 解析的所有目标 IP，再连接到经过校验的具体地址，阻止 DNS 重绑定到内网。HTTP(S) 之外的网络默认不可用。

平台自身访问 Docker socket，属于受信任的管理组件。上传权限仅授予管理员。此版本针对自用工具，不是面向公开用户的多租户任意代码托管服务。

状态、工具包、输入文件、产物在 tooldeck_data 卷。运行时镜像是独立构建产物。对外访问部署需使用 HTTPS 反向代理。OAuth introspection URL 由部署管理员配置，使用 HTTPS；不接受调用方指定任意验证地址。

后续可将 Store 替换为数据库及对象存储，并拆分 Worker 的领取、心跳和完成接口。当前不暴露未经实现的远程节点接入接口。
