# ReActicle

**AI 时代的 HTML 文章协议** — 让 AI 不直接手写 HTML，而是用受限、语义化的 React 组件，生成稳定、漂亮、可交互、自包含的 HTML 文章与报告。

> Markdown 让人类写作变得轻量，ReActicle 让 AI 生成 HTML 变得可控。

当前工程是**单包组件库 + 统一站点**：

- **组件库**：根包 `reacticle`，`src/` 只放可发布的 React 组件、主题、导出工具和公共入口。
- **统一站点**：`apps/site`，把文档、组件 live 参考与 Gallery 合并成一个应用——一套视觉语言、一次部署。

## 快速开始

```bash
npm install
npm run dev       # 统一站点：文档 · 组件参考 · Gallery
```

## 命令

| 命令 | 作用 |
| --- | --- |
| `npm run dev` | 启动统一站点（文档 / 组件 / Gallery） |
| `npm run build:lib` | 构建组件库产物 |
| `npm run build:site` | 构建统一站点 |
| `npm run build:report` | 构建自包含单页 HTML 报告 |
| `npm run build` | 依次构建组件库与站点 |
| `npm run preview:site` | 预览站点构建产物 |
| `npm run typecheck` | 全工程类型检查 |

## 工程结构

```text
src/
  components/            # 语义组件
  theme/                 # token 契约、主题、打印样式
  export/                # PDF/复制等导出能力
  index.ts               # 唯一公共入口

apps/
  site/                  # 统一站点：文档 + 组件参考 + Gallery + 单页报告构建

skill/                   # AI authoring skill
docs/architecture/       # 工程架构文档
```

完整架构说明见 [工程架构文档](docs/architecture/engineering-architecture.md)。
Markdown 覆盖对照与开发计划见 [ReActicle vs Markdown 覆盖清单](docs/markdown-coverage-plan.md)。

## 组件库

组件库就是仓库根包 `reacticle`。外部消费者只允许从包入口导入：

```tsx
import { ThemeProvider, Article, Hero, Section } from "reacticle";
```

组件分组：

| 类别 | 组件 |
| --- | --- |
| 结构 | Article, Hero, Lead, Section, Subsection, TOC |
| 观点 | Summary, Aside, Quote |
| 媒体 | Image, Video, Audio |
| 数据 | Table |
| 决策 | RiskList, Decision, ActionList, Checkpoint, Tradeoff, Incident |
| 技术 | CodeBlock, Formula, DiffReview |
| 交互壳 | Detail, Tabs |
| 自由层 | Raw |

主题：当前注册 `tufte`（Edward Tufte / Data-Ink）与 `press`（Stripe Press / 书卷长读物）。新增主题只改 `src/theme/themes/<name>/` 与 `ThemeProvider` 的主题注册。

每套主题不仅是 CSS token，也是一份**生成风格契约**。主题目录里的 `<name>.md`
必须写清：

- **代码与公式风格**：`CodeBlock` / `Formula` 的高亮气质、行号、公式留白和 token 色彩如何匹配主题。
- **媒体风格**：`Image` / `Video` / `Audio` 应选择什么摄影、截图、图表、视频、音频或纹理，禁止什么素材气质。
- **Raw 自由层风格**：AI 在 `Raw` 中手写 SVG、交互、动画、一次性 HTML/CSS 时，应采用什么构图、动效、线条、密度和 token 使用方式。

后续 AI authoring 必须先读取所选主题的 md，再选择图片和编写 Raw；不能生成与主题气质冲突的视觉内容。

## 统一站点（apps/site）

`apps/site` 把文档、组件 live 参考与 Gallery 合并成同一个应用，作为真实消费者只从 `reacticle` 公共入口使用组件，不引用库内部目录。设计语言为 **Editorial Console**：暖墨/纸双模式（Paper / Ink）、编辑级衬线 + monospace UI、发丝线工程感、单一 Tufte 朱红强调色，调色板直接复用库自带的 Tufte token。

站点结构：

- `概览`：协议主张、live 迷你文章、设计原则与组件目录
- `开始使用`：安装、最小骨架、构建与导出、缺失即可见
- `组件`：每个组件的 live 预览 + 属性表 + 可复制代码
- `主题`：token 契约、tufte 主题与新增主题路径
- `Gallery`：真实报告预览与主题切换
- `架构`：工程边界与依赖原则

报告与单页构建入口：

- `apps/site/src/reports/*.tsx`：Gallery 文章组件（每个主题一篇），由 `apps/site/src/data/gallery.tsx` 注册
- `apps/site/report.html` + `apps/site/src/reports/report-entry.tsx`：自包含报告构建入口

构建自包含单页 HTML：

```bash
npm run build:report
```

产物输出到 `dist-report/report.html`，CSS 与 JS 内联，断网可打开。

## 边界原则

1. 组件库不依赖站点。
2. 站点只能通过 `reacticle` 公共入口消费组件。
3. 主题属于组件库；主题预览与主题说明都在统一站点。
4. 示例文章属于站点，不放进组件库包。
5. 架构文档放在 `docs/architecture`，不混进组件 API 文档。

## License

MIT
