# 阅微 PC 同步（ReaMicro PC）

一个用于「阅微」安卓阅读 App 的 **PC 端局域网同步工具**，使用 **Go + Wails + Vue 3** 构建。

通过完全复用阅微 App 内置的「设备同步」功能（基于 mDNS + 自定义 TCP 协议），把电脑上的 EPUB 书籍推送到局域网内的安卓阅微设备。**无需改动 App、无需登录账号（PC 端）**。

## 功能

- **免登录**：PC 端只需本机账号配置，无需在 PC 上登录阅微
- **mDNS 广播**：安卓端「设备同步」自动发现本机（设备名按平台显示 `Mac` / `Win`）
- **自动解包**：EPUB 自动解包为阅微可识别的目录结构（`META-INF/`、`content.opf` 等），导入后可直接阅读
- **封面提取**：解析 OPF 提取封面，安卓书架显示真实封面
- **完整同步协议**：HELLO / MANIFEST / GET_FILE / GET_FILES / GET_BOOK_FILES
- **精简日志**：同步完成后汇总显示「N 个文件 / X MB」

## 技术栈

- **Go**：同步协议、mDNS、TCP 服务器、EPUB 处理、登录 API
- **Wails v2**：桌面应用框架（WebView 渲染 UI）
- **Vue 3 + TypeScript + Vite**：前端界面
- **hashicorp/mdns**：mDNS 广播

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
2. 选择存放 `.epub` 的目录
3. 启动同步服务
4. 手机打开阅微「设置 → 设备同步」，选择本机（`Mac` / `Win`）
5. 勾选书籍同步，导入后即可阅读，封面正常显示

## 项目结构

```
├── main.go          # Wails 入口
├── app.go           # 前端绑定方法（登录/扫描/启动/停止）
├── wire.go          # 同步协议编码与数据模型
├── host.go          # TCP host 服务器
├── mdns.go          # mDNS 广播
├── library.go       # EPUB 解包 / 封面提取 / 书籍扫描
├── login.go         # 登录 API
├── flexjson.go      # 宽松 JSON 数字解析
├── config.go        # 配置持久化
└── frontend/        # Vue 3 前端
```

## 协议说明

逆向自阅微 APK（`app.zhendong.reamicro.lan.sync` 包），与安卓端源码逻辑一致。设备同步走自定义 TCP 协议（文本控制行 + 二进制 body），非 HTTP。
