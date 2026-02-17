<div align="center">

# 节点延迟测试

**一款快速、现代化的代理节点延迟测试桌面工具**

[![Go 版本][go-version-badge]][go-download]
[![开源协议][license-badge]][license]
[![构建状态][build-badge]][github-actions]
[![GitHub Stars][stars-badge]][github-repo]

<img src="logo.png" alt="Node Latency Logo" width="200"/>

*导入节�� · 测试延迟 · 导出优选 — 尽在一款精美的图形界面*

[English](./README.md) · 简体中文

</div>

---

## ✨ 功能特性

采用前沿技术栈打造，追求极致性能与开发体验：

- **[Go][go-official]** — 极速编译型后端，并发性能卓越
- **[Wails][wails]** — 轻量级桌面应用框架，原生性能体验
- **[Vue 3][vue]** — 现���化响应式前端框架
- **[TypeScript][typescript]** — 类型安全的 JavaScript，提升开发体验
- **[TailwindCSS][tailwind]** — 实用优先的 CSS 框架
- **[Pinia][pinia]** — Vue 官方推荐的状态管理库

### 核心能力

- 🚀 **多协议支持** — VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC、SOCKS5、HTTP
- 📥 **灵活导入** — 支持订阅链接、本地文件、直接粘贴等多种导入方式
- ⚡ **Mihomo 驱动** — 基于 Mihomo（Clash Meta）核心，延迟测试精准可靠
- 📊 **实时反馈** — 测试进度实时更新，结果详尽直观
- 📤 **智能导出** — 将通过测试的节点导出为 Clash 配置或分享链接
- 🎨 **现代界面** — 简洁美观的用户界面，虚拟滚动支持大规模节点列表

---

## ⚡️ 快速开始

### 环境要求

- **Go 1.24+** — [下载 Go][go-download]
- **Node.js 18+** — [下载 Node.js][node-download]
- **Wails CLI** — 执行 `go install github.com/wailsapp/wails/v2/cmd/wails@latest` 安装

### 安装运行

```bash
# 克隆仓库
git clone https://github.com/Poppypy/node-latency.git
cd node-latency

# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式运行
wails dev
```

### 构建生产版本

```bash
# 构建可执行文件
wails build
```

编译后的程序位于 `build/bin/` 目录。

---

## 📖 使用指南

1. **导入节点** — 输入订阅地址、直接粘贴节点链接，或从本地文件导入
2. **配置参数** — 设置测试 URL、超时时间和并发数
3. **开始测试** — 点击测试按钮，实时查看测试结果
4. **导出结果** — 按延迟筛选节点，导出为 Clash 配置或节点链接

### 支持的协议

| 协议 | 链接格式 | Clash 配置 |
|------|----------|------------|
| VLESS | `vless://...` | ✅ |
| VMess | `vmess://...` | ✅ |
| Trojan | `trojan://...` | ✅ |
| Shadowsocks | `ss://...` | ✅ |
| Hysteria2 | `hysteria2://...` | ✅ |
| TUIC | `tuic://...` | ✅ |
| SOCKS5 | `socks5://...` | ✅ |
| HTTP | `http://...` | ✅ |

---

## 🤝 参与贡献

欢迎并感谢任何形式的贡献！无论是 Bug 反馈、功能建议还是代码提交 — 我们期待您的参与。

1. **Fork** 本仓库
2. **创建** 功能分支 (`git checkout -b feature/amazing-feature`)
3. **提交** 更改 (`git commit -m 'Add some amazing feature'`)
4. **推送** 分支 (`git push origin feature/amazing-feature`)
5. **发起** Pull Request

发现问题？有好的建议？[提交 Issue][github-issues] 告诉我们！

---

## ⚠️ 开源协议

本项目基于 **MIT 协议** 开源 — 详见 [LICENSE][license] 文件。

---

<div align="center">

**由 [Popy][github-author] 用 ❤️ 构建**

如果这个项目对你有帮助，请给一个 ⭐️！

[⬆ 返回顶部](#节点延迟测试)

</div>

<!-- Reference-style Links -->

[go-version-badge]: https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go
[license-badge]: https://img.shields.io/badge/协议-MIT-green?style=flat-square
[build-badge]: https://img.shields.io/badge/构建-通过-brightgreen?style=flat-square
[stars-badge]: https://img.shields.io/github/stars/Poppypy/node-latency?style=flat-square&logo=github

[go-official]: https://golang.org
[go-download]: https://golang.org/dl/
[wails]: https://wails.io
[vue]: https://vuejs.org
[typescript]: https://www.typescriptlang.org
[tailwind]: https://tailwindcss.com
[pinia]: https://pinia.vuejs.org
[node-download]: https://nodejs.org/

[license]: ./LICENSE
[github-repo]: https://github.com/Poppypy/node-latency
[github-issues]: https://github.com/Poppypy/node-latency/issues
[github-actions]: https://github.com/Poppypy/node-latency/actions
[github-author]: https://github.com/Poppypy
