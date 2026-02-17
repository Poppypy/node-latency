<div align="center">

# Node Latency

**A fast, modern desktop tool for testing proxy node latency with ease**

[![Go Version][go-version-badge]][go-download]
[![License][license-badge]][license]
[![Build][build-badge]][github-actions]
[![GitHub Stars][stars-badge]][github-repo]

<img src="logo.png" alt="Node Latency Logo" width="200"/>

*Import nodes, test latency, export the best — all in one beautiful GUI*

[English](#) · [简体中文](#)

</div>

---

## ✨ Features

Built with cutting-edge technologies for maximum performance and developer experience:

- **[Go][go-official]** — Blazing fast, compiled backend with excellent concurrency
- **[Wails][wails]** — Lightweight desktop apps with native performance
- **[Vue 3][vue]** — Modern, reactive frontend framework
- **[TypeScript][typescript]** — Type-safe JavaScript for better DX
- **[TailwindCSS][tailwind]** — Utility-first CSS framework
- **[Pinia][pinia]** — Intuitive state management for Vue

### Core Capabilities

- 🚀 **Multi-Protocol Support** — VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, SOCKS5, HTTP
- 📥 **Flexible Import** — Import nodes from subscriptions, files, or paste directly
- ⚡ **Mihomo-Powered Testing** — Accurate latency testing using the Mihomo (Clash Meta) core
- 📊 **Real-Time Results** — Live progress updates and detailed test results
- 📤 **Smart Export** — Export passing nodes as Clash YAML or shareable links
- 🎨 **Modern UI** — Clean, responsive interface with virtual scrolling for large node lists

---

## ⚡️ Quick start

### Prerequisites

- **Go 1.24+** is required — [Download Go][go-download]
- **Node.js 18+** is required — [Download Node.js][node-download]
- **Wails CLI** — Install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Installation

```bash
# Clone the repository
git clone https://github.com/Poppypy/node-latency.git
cd node-latency

# Install frontend dependencies
cd frontend && npm install && cd ..

# Run in development mode
wails dev
```

### Build for Production

```bash
# Build a production-ready executable
wails build
```

The compiled binary will be available in the `build/bin/` directory.

---

## 📖 Usage

1. **Import Nodes** — Enter a subscription URL, paste node links directly, or import from a local file
2. **Configure Settings** — Set the test URL, timeout, and concurrency level
3. **Start Testing** — Click the test button and watch results stream in real-time
4. **Export Results** — Filter nodes by latency and export as Clash config or node links

### Supported Formats

| Protocol | URL Scheme | Config Format |
|----------|------------|---------------|
| VLESS | `vless://...` | ✅ |
| VMess | `vmess://...` | ✅ |
| Trojan | `trojan://...` | ✅ |
| Shadowsocks | `ss://...` | ✅ |
| Hysteria2 | `hysteria2://...` | ✅ |
| TUIC | `tuic://...` | ✅ |
| SOCKS5 | `socks5://...` | ✅ |
| HTTP | `http://...` | ✅ |

---

## 🤝 Contributing

Contributions are welcome and greatly appreciated! Whether it's bug reports, feature requests, or code contributions — we'd love to have you involved.

1. **Fork** the repository
2. **Create** your feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add some amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

Found a bug? Have a suggestion? [Open an issue][github-issues] and let us know!

---

## ⚠️ License

This project is licensed under the **MIT License** — see the [LICENSE][license] file for details.

---

<div align="center">

**Made with ❤️ by [Popy][github-author]**

If you find this project helpful, consider giving it a ⭐️!

[⬆ Back to top](#node-latency)

</div>

<!-- Reference-style Links -->

[go-version-badge]: https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go
[license-badge]: https://img.shields.io/badge/License-MIT-green?style=flat-square
[build-badge]: https://img.shields.io/badge/Build-Passing-brightgreen?style=flat-square
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
