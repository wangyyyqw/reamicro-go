# 阅微 PC 同步（ReaMicro PC）

一个用于「阅微」安卓阅读 App 的 **PC 端局域网同步工具**，使用 **Go + Wails + Vue 3** 构建。

通过完全复用阅微 App 内置的「设备同步」功能（基于 mDNS + 自定义 TCP 协议），把电脑上的 EPUB 书籍推送到局域网内的安卓阅微设备。**无需改动 App**，PC 端也无需在 App 中登录（仅在首次同步时用邮箱验证码绑定账号）。

## 功能

- **免安装改**：不改动阅微 App 任何代码，完全复用其「设备同步」协议
- **mDNS 广播**：安卓端「设置 → 设备同步」自动发现本机，设备名按平台显示 `Mac` / `Win`
- **自动解包**：EPUB 自动解包为阅微可识别的目录结构（`META-INF/container.xml`、`content.opf` 等），导入后可直接阅读
- **封面提取**：解析 OPF 按三级策略提取封面（`cover-image` 属性 → `meta cover` → 首个图片），安卓书架显示真实封面
- **完整同步协议**：HELLO / MANIFEST / GET_BOOK_FILES / GET_FILE / GET_FILES 全链路
- **账号校验**：HELLO 握手校验 userId 一致，不同账号设备拒绝同步
- **安全防护**：路径穿越防护（zip 解包与文件读取双重校验）
- **精简日志**：同步完成后汇总显示「N 个文件 / X MB」，实时事件推送至前端

## 工作原理

```
 ┌──────────────┐   mDNS 广播      ┌────────────────┐
 │  PC 端 (本工具) │ ◄─────────────► │  安卓阅微 App    │
 │              │  服务发现         │  设备同步        │
 │  TCP Server  │ ◄──────────────► │                │
 │  解包后的 EPUB│  自定义线协议     │  Opf.obtain()  │
 └──────────────┘                  └────────────────┘
```

1. PC 端启动 TCP 服务（随机端口）+ mDNS 广播 `ReaMicroSync-<userId>-<suffix6>-<label>`
2. 安卓端「设备同步」通过 mDNS 发现本机，建立 TCP 连接并发起 `HELLO` 握手
3. 校验 userId 一致后，安卓端请求 `MANIFEST` 获取书籍清单
4. 按需通过 `GET_BOOK_FILES` / `GET_FILE` / `GET_FILES` 拉取解包后的文件
5. 安卓端对书目录直接执行 `Opf.obtain()` 导入，封面、正文即可正常使用

## 技术栈

- **Go**：同步协议、mDNS、TCP 服务器、EPUB 处理、登录 API
- **Wails v2**：桌面应用框架（WebView 渲染 UI）
- **Vue 3 + TypeScript + Vite**：前端界面
- **hashicorp/mdns**：mDNS 广播

## 项目结构

```
├── main.go          # Wails 入口，嵌入 frontend/dist 并绑定 App
├── app.go           # Wails 绑定方法：登录 / 会话 / 扫描 / 选目录 / 启停服务
├── wire.go          # 同步协议：命令常量、JSON 数据模型、线协议编解码
├── host.go          # TCP host 服务器：握手校验、命令分发、同步统计
├── mdns.go          # mDNS 广播（绑定真实网卡，设备名按平台显示）
├── library.go       # EPUB 扫描 / 解包缓存 / OPF 解析 / 封面提取
├── login.go         # 邮箱验证码登录 API（api.reamicro.zhendong.ltd）
├── flexjson.go      # 宽松 JSON 数字解析（服务器类型不稳定）
├── config.go        # 配置持久化、局域网 IP/接口选择
└── frontend/        # Vue 3 前端
```

## 数据存储

| 路径 | 用途 |
| --- | --- |
| `~/.reamicro_pc/config.json` | 登录态与配置（token、userId、昵称、书籍目录、deviceId） |
| `~/.reamicro_pc/unpacked/<md5>/` | EPUB 解包缓存（按文件内容 MD5 命名，同步停止时清理） |

## 开发

前置要求：Go 1.25+、Node.js 20+

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 安装前端依赖
cd frontend && npm install

# 实时开发（热重载）
wails dev

# 构建生产版本
wails build
```

构建产物在 `build/bin/`。

## 使用

1. 打开 `ReaMicroPC.app`
2. 邮箱验证码登录（绑定阅微账号）
3. 选择存放 `.epub` 的目录
4. 启动同步服务
5. 手机打开阅微「设置 → 设备同步」，选择本机（`Mac` / `Win`）
6. 勾选书籍同步，导入后即可阅读，封面正常显示

## 同步协议

逆向自阅微 APK（`app.zhendong.reamicro.lan.sync` 包），与安卓端源码逻辑一致。走自定义 TCP 协议，非 HTTP。

### 线协议

每帧为一行控制行 + 可选 body：

```
CMD            # 无 body
CMD <len>      # 后接 len 字节的二进制/JSON body
```

body 有两种格式：

- **JSON**：HELLO、HELLO_OK、MANIFEST、BOOK_FILES 等结构化数据
- **二进制**：FILES 用 `packFiles` 打包，结构为 `u32 count` + 每条 `u32 nameLen + name + u32 dataLen + data`（大端序）

### 命令

| 命令 | 方向 | 说明 |
| --- | --- | --- |
| `HELLO` | 端 → PC | 握手，携带 userId/deviceId，校验账号一致 |
| `HELLO_OK` | PC → 端 | 握手成功，回传 PC 设备信息 |
| `MANIFEST` | PC → 端 | 书籍清单（BookSnapshot + Marks + Files） |
| `GET_BOOK_FILES` | 端 → PC | 批量查询指定 uuid 的书内文件列表 |
| `BOOK_FILES` | PC → 端 | 响应文件列表 |
| `GET_FILE` | 端 → PC | 拉取单文件（二进制 body） |
| `GET_FILES` | 端 → PC | 批量拉取文件（packFiles 打包） |
| `FILE` / `FILES` | PC → 端 | 文件内容响应 |
| `ERR` | 双向 | 错误响应 |
| `BYE` | 端 → PC | 结束会话 |

### mDNS

- 服务类型：`_reamicro-sync._tcp`
- 实例名：`ReaMicroSync-<userId>-<deviceSuffix6>-<label>`（label 为平台名 `Mac`/`Win`/`Linux`/`PC`）
- TXT 记录：`userId`、`deviceId`、`nickname`
- 显式绑定真实局域网接口，排除代理 fake-ip（`198.18.x`/`198.19.x`）与链路本地（`169.254.x`）网段

## 发布

推送形如 `v1.0.0` 的 tag 会自动触发 GitHub Actions 构建并发布 Release：

```bash
git tag v1.0.0
git push origin v1.0.0
```

- **macOS**：`darwin/universal`（Intel + Apple Silicon 通用），打包为 zip
- **Windows**：`windows/amd64`，打包为 zip
- 产物未签名，macOS 首次打开需右键 → 打开（或「系统设置 → 隐私与安全性」中允许）

## 注意事项

- 同步服务对局域网内可被发现的设备开放，请勿在不可信网络中开启
- 解包缓存每次停止服务时清空，下次同步自动按内容 MD5 重新解包
