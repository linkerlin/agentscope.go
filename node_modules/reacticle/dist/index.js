import { jsx as y, jsxs as $, Fragment as Ya } from "react/jsx-runtime";
import { useState as ze, useEffect as Wa, useRef as Za, useLayoutEffect as Ka, createContext as Ja, useId as Pt, useContext as Qa, useMemo as en } from "react";
const Fs = [
  "tufte",
  "press",
  "shannon",
  "vignelli",
  "knuth",
  "freddie",
  "andy",
  "bodoni",
  "bayer",
  "fuller",
  "sottsass"
], Ns = {
  tufte: "Tufte · Data-Ink",
  press: "Press · 书卷",
  shannon: "Shannon · 工程暗色",
  vignelli: "Vignelli · 瑞士",
  knuth: "Knuth · 学术",
  freddie: "Freddie · 暖黄",
  andy: "Andy · 静谧",
  bodoni: "Bodoni · 报刊",
  bayer: "Bayer · 包豪斯",
  fuller: "Fuller · 蓝图",
  sottsass: "Sottsass · 孟菲斯"
};
function Cs({
  theme: r = "tufte",
  children: e,
  className: t
}) {
  return /* @__PURE__ */ y(
    "div",
    {
      className: t ? `ra-root ${t}` : "ra-root",
      "data-theme": r,
      children: e
    }
  );
}
function tn({ items: r, title: e = "目录" }) {
  const [t, a] = ze(null);
  return Wa(() => {
    if (r.length === 0 || typeof IntersectionObserver > "u") return;
    const n = r.map((u) => document.getElementById(u.id)).filter((u) => u != null);
    if (n.length === 0) return;
    const s = new IntersectionObserver(
      (u) => {
        const o = u.filter((h) => h.isIntersecting).sort((h, m) => h.boundingClientRect.top - m.boundingClientRect.top);
        o[0] && a(o[0].target.id);
      },
      { rootMargin: "0px 0px -70% 0px", threshold: 0 }
    );
    return n.forEach((u) => s.observe(u)), () => s.disconnect();
  }, [r]), r.length === 0 ? null : /* @__PURE__ */ $("nav", { className: "ra-toc", "aria-label": e, children: [
    /* @__PURE__ */ y("div", { className: "ra-toc__title", children: e }),
    /* @__PURE__ */ y("ol", { className: "ra-toc__list", children: r.map((n) => {
      const u = ["ra-toc__item", `ra-toc__item--l${n.level ?? 1}`];
      return t === n.id && u.push("ra-toc__item--active"), /* @__PURE__ */ y("li", { className: u.join(" "), children: /* @__PURE__ */ $("a", { href: `#${n.id}`, children: [
        n.index ? /* @__PURE__ */ y("span", { className: "ra-toc__index", children: n.index }) : null,
        /* @__PURE__ */ y("span", { className: "ra-toc__label", children: n.title ?? "未命名章节" })
      ] }) }, n.id);
    }) })
  ] });
}
function rn(r, e) {
  return r.length !== e.length ? !1 : r.every((t, a) => {
    const n = e[a];
    return n != null && t.id === n.id && t.index === n.index && t.title === n.title && t.level === n.level;
  });
}
function Es({ children: r, toc: e = !1, width: t = "regular" }) {
  const a = Za(null), [n, s] = ze([]);
  Ka(() => {
    if (!e) {
      n.length && s([]);
      return;
    }
    const o = a.current;
    if (!o) return;
    const h = Array.from(
      o.querySelectorAll("[data-ra-toc]")
    ).map((m) => {
      const v = Number(m.getAttribute("data-ra-level")) || 1;
      return {
        id: m.id,
        index: m.getAttribute("data-ra-index") ?? void 0,
        title: m.getAttribute("data-ra-title") ?? void 0,
        level: Math.min(Math.max(v, 1), 3)
      };
    });
    s((m) => rn(m, h) ? m : h);
  });
  const u = e && n.length > 0;
  return /* @__PURE__ */ $(
    "div",
    {
      className: u ? "ra-article-layout ra-article-layout--with-toc" : "ra-article-layout",
      "data-width": t,
      children: [
        u ? /* @__PURE__ */ y(tn, { items: n }) : null,
        /* @__PURE__ */ y("article", { className: "ra-article", ref: a, children: r })
      ]
    }
  );
}
function J({ label: r }) {
  return /* @__PURE__ */ $("span", { className: "ra-missing", role: "alert", children: [
    "未指定",
    r
  ] });
}
function c0(r, e) {
  return r == null || r === "" ? /* @__PURE__ */ y(J, { label: e }) : r;
}
function Ds({ title: r, eyebrow: e, subtitle: t, meta: a }) {
  return /* @__PURE__ */ $("header", { className: "ra-hero", children: [
    e ? /* @__PURE__ */ y("div", { className: "ra-hero__eyebrow", children: e }) : null,
    /* @__PURE__ */ y("h1", { className: "ra-hero__title", children: c0(r, "标题") }),
    t ? /* @__PURE__ */ y("div", { className: "ra-hero__subtitle", children: t }) : null,
    a && a.length > 0 ? /* @__PURE__ */ y("div", { className: "ra-hero__meta", children: a.map((n, s) => /* @__PURE__ */ $("span", { className: "ra-hero__meta-item", children: [
      /* @__PURE__ */ $("strong", { children: [
        n.label,
        "："
      ] }),
      n.value
    ] }, s)) }) : null
  ] });
}
function Bs({ children: r }) {
  return /* @__PURE__ */ y("p", { className: "ra-lead", children: r });
}
const wt = Ja(1), an = 3;
function Vr(r, e, t, a, n) {
  if (e) return e;
  const u = (t ?? a ?? "").trim().toLowerCase().replace(/\s+/g, "-").replace(/[^\p{L}\p{N}_-]+/gu, "-").replace(/-+/g, "-").replace(/^-|-$/g, "");
  return u ? `${r}-${u}` : `${r}-${n}`;
}
function Is({ title: r, index: e, id: t, children: a }) {
  const n = Pt().replace(/[^a-zA-Z0-9_-]/g, ""), s = Vr("section", t, e, r, n);
  return /* @__PURE__ */ $(
    "section",
    {
      className: "ra-section",
      id: s,
      "data-ra-toc": "",
      "data-ra-level": "1",
      ...e ? { "data-ra-index": e } : {},
      ...r ? { "data-ra-title": r } : {},
      children: [
        /* @__PURE__ */ $("header", { className: "ra-section__head", children: [
          e ? /* @__PURE__ */ y("span", { className: "ra-section__index", children: e }) : null,
          /* @__PURE__ */ y("h2", { className: "ra-section__title", children: c0(r, "章节标题") })
        ] }),
        /* @__PURE__ */ y("div", { className: "ra-section__body", children: /* @__PURE__ */ y(wt.Provider, { value: 1, children: a }) })
      ]
    }
  );
}
function Rs({
  title: r,
  index: e,
  id: t,
  level: a,
  children: n
}) {
  const s = Qa(wt), u = a ?? Math.min(s + 1, an), o = Pt().replace(/[^a-zA-Z0-9_-]/g, ""), h = Vr("sub", t, e, r, o), m = u >= 3 ? "h4" : "h3";
  return /* @__PURE__ */ $(
    "section",
    {
      className: `ra-subsection ra-subsection--l${u}`,
      id: h,
      "data-ra-toc": "",
      "data-ra-level": u,
      ...e ? { "data-ra-index": e } : {},
      ...r ? { "data-ra-title": r } : {},
      children: [
        /* @__PURE__ */ $("header", { className: "ra-subsection__head", children: [
          e ? /* @__PURE__ */ y("span", { className: "ra-subsection__index", children: e }) : null,
          /* @__PURE__ */ y(m, { className: "ra-subsection__title", children: c0(r, "小节标题") })
        ] }),
        /* @__PURE__ */ y("div", { className: "ra-subsection__body", children: /* @__PURE__ */ y(wt.Provider, { value: u, children: n }) })
      ]
    }
  );
}
function qs({ title: r = "结论", children: e, takeaways: t }) {
  const a = t != null && t.length > 0, n = !e && !a;
  return /* @__PURE__ */ $("section", { className: "ra-conclusion", children: [
    /* @__PURE__ */ y("div", { className: "ra-conclusion__label", children: r }),
    e ? /* @__PURE__ */ y("div", { className: "ra-conclusion__body", children: e }) : null,
    a ? /* @__PURE__ */ y("ul", { className: "ra-conclusion__takeaways", children: t.map((s, u) => /* @__PURE__ */ y("li", { children: s }, u)) }) : null,
    n ? /* @__PURE__ */ y(J, { label: "结论内容" }) : null
  ] });
}
function _s({ points: r, title: e = "摘要" }) {
  const t = r != null && r.length > 0;
  return /* @__PURE__ */ $("div", { className: "ra-card ra-summary", children: [
    /* @__PURE__ */ y("span", { className: "ra-card__label", children: e }),
    t ? /* @__PURE__ */ y("ul", { className: "ra-summary__list", children: r.map((a, n) => /* @__PURE__ */ y("li", { children: a }, n)) }) : /* @__PURE__ */ y(J, { label: "摘要要点" })
  ] });
}
const nn = {
  note: "补充说明",
  principle: "原则",
  capability: "能力",
  warning: "注意"
};
function Os({ tone: r = "note", label: e, children: t }) {
  return /* @__PURE__ */ $("aside", { className: `ra-card ra-aside ra-aside--${r}`, children: [
    /* @__PURE__ */ y("span", { className: "ra-card__label ra-aside__label", children: e ?? nn[r] }),
    /* @__PURE__ */ y("div", { className: "ra-aside__body", children: t || /* @__PURE__ */ y(J, { label: "旁注内容" }) })
  ] });
}
function Ls({ children: r, who: e, source: t }) {
  const a = [e, t].filter(Boolean).join("，");
  return /* @__PURE__ */ $("figure", { className: "ra-quote", children: [
    /* @__PURE__ */ y("blockquote", { className: "ra-quote__text", children: r || /* @__PURE__ */ y(J, { label: "引文" }) }),
    a ? /* @__PURE__ */ y("figcaption", { className: "ra-quote__by", children: a }) : null
  ] });
}
function sn(r, e) {
  return r == null || r === "" ? /* @__PURE__ */ y(J, { label: e }) : r;
}
function Hs({ caption: r, source: e, columns: t, rows: a }) {
  const n = t != null && t.length > 0 && a != null && a.length > 0;
  return /* @__PURE__ */ $("figure", { className: "ra-table", children: [
    r ? /* @__PURE__ */ y("figcaption", { className: "ra-table__caption", children: r }) : null,
    n ? /* @__PURE__ */ y("div", { className: "ra-table__scroll", children: /* @__PURE__ */ $("table", { className: "ra-table__table", children: [
      /* @__PURE__ */ y("colgroup", { children: t.map((s) => /* @__PURE__ */ y("col", { style: s.width ? { width: s.width } : void 0 }, s.key)) }),
      /* @__PURE__ */ y("thead", { children: /* @__PURE__ */ y("tr", { children: t.map((s) => /* @__PURE__ */ y("th", { className: `ra-table__cell--${s.align ?? "left"}`, children: s.label ?? s.key }, s.key)) }) }),
      /* @__PURE__ */ y("tbody", { children: a.map((s, u) => /* @__PURE__ */ y("tr", { children: t.map((o) => /* @__PURE__ */ y("td", { className: `ra-table__cell--${o.align ?? "left"}`, children: sn(s[o.key], o.label ?? o.key) }, o.key)) }, u)) })
    ] }) }) : /* @__PURE__ */ y("div", { className: "ra-table__missing", children: /* @__PURE__ */ y(J, { label: "表格列与行" }) }),
    e ? /* @__PURE__ */ y("div", { className: "ra-table__source", children: e }) : null
  ] });
}
function $s({ src: r, alt: e, caption: t, credit: a, width: n, ratio: s }) {
  const u = e == null || e === "", o = r != null && /^\s*data:image\/svg/i.test(r), h = r != null && r !== "" && !o;
  return /* @__PURE__ */ $("figure", { className: "ra-image", children: [
    /* @__PURE__ */ y(
      "div",
      {
        className: "ra-image__frame",
        style: n ? { maxWidth: n } : void 0,
        children: h ? /* @__PURE__ */ y(
          "img",
          {
            className: "ra-image__img",
            src: r,
            alt: u ? "" : e,
            loading: "lazy",
            style: s ? { aspectRatio: s } : void 0
          }
        ) : /* @__PURE__ */ y("div", { className: "ra-image__placeholder", children: o ? /* @__PURE__ */ y("span", { className: "ra-missing", role: "alert", children: "Image 只接受真实图片 URL —— 内联 SVG 请改用 Raw" }) : /* @__PURE__ */ y(J, { label: "图片地址 src" }) })
      }
    ),
    h && u ? /* @__PURE__ */ y("div", { className: "ra-image__alt-missing", children: /* @__PURE__ */ y(J, { label: "替代文本 alt" }) }) : null,
    t || a ? /* @__PURE__ */ $("figcaption", { className: "ra-image__caption", children: [
      t ? /* @__PURE__ */ y("span", { className: "ra-image__text", children: t }) : null,
      a ? /* @__PURE__ */ y("span", { className: "ra-image__credit", children: a }) : null
    ] }) : null
  ] });
}
function Ps({
  src: r,
  title: e,
  poster: t,
  caption: a,
  credit: n,
  width: s,
  ratio: u,
  controls: o = !0,
  preload: h = "metadata",
  autoPlay: m,
  muted: v,
  loop: b,
  playsInline: k = !0
}) {
  const S = r != null && r !== "", _ = e == null || e === "";
  return /* @__PURE__ */ $("figure", { className: "ra-media ra-video", children: [
    /* @__PURE__ */ y(
      "div",
      {
        className: "ra-media__frame",
        style: s ? { maxWidth: s } : void 0,
        children: S ? /* @__PURE__ */ y(
          "video",
          {
            className: "ra-video__player",
            src: r,
            poster: t,
            controls: o,
            preload: h,
            autoPlay: m,
            muted: v,
            loop: b,
            playsInline: k,
            "aria-label": _ ? a ?? "视频" : e,
            style: u ? { aspectRatio: u } : void 0
          }
        ) : /* @__PURE__ */ y("div", { className: "ra-media__placeholder", children: /* @__PURE__ */ y(J, { label: "视频地址 src" }) })
      }
    ),
    S && _ ? /* @__PURE__ */ y("div", { className: "ra-media__missing", children: /* @__PURE__ */ y(J, { label: "媒体标题 title" }) }) : null,
    a || n ? /* @__PURE__ */ $("figcaption", { className: "ra-media__caption", children: [
      a ? /* @__PURE__ */ y("span", { className: "ra-media__text", children: a }) : null,
      n ? /* @__PURE__ */ y("span", { className: "ra-media__credit", children: n }) : null
    ] }) : null
  ] });
}
function Gs({
  src: r,
  title: e,
  caption: t,
  credit: a,
  width: n,
  controls: s = !0,
  preload: u = "metadata",
  autoPlay: o,
  loop: h
}) {
  const m = r != null && r !== "", v = e == null || e === "";
  return /* @__PURE__ */ $("figure", { className: "ra-media ra-audio", children: [
    /* @__PURE__ */ y(
      "div",
      {
        className: "ra-media__frame ra-audio__frame",
        style: n ? { maxWidth: n } : void 0,
        children: m ? /* @__PURE__ */ y(
          "audio",
          {
            className: "ra-audio__player",
            src: r,
            controls: s,
            preload: u,
            autoPlay: o,
            loop: h,
            "aria-label": v ? t ?? "音频" : e
          }
        ) : /* @__PURE__ */ y("div", { className: "ra-media__placeholder", children: /* @__PURE__ */ y(J, { label: "音频地址 src" }) })
      }
    ),
    m && v ? /* @__PURE__ */ y("div", { className: "ra-media__missing", children: /* @__PURE__ */ y(J, { label: "媒体标题 title" }) }) : null,
    t || a ? /* @__PURE__ */ $("figcaption", { className: "ra-media__caption", children: [
      t ? /* @__PURE__ */ y("span", { className: "ra-media__text", children: t }) : null,
      a ? /* @__PURE__ */ y("span", { className: "ra-media__credit", children: a }) : null
    ] }) : null
  ] });
}
const ln = {
  high: "高",
  medium: "中",
  low: "低"
};
function St({ prefix: r, level: e }) {
  return /* @__PURE__ */ $("span", { className: `ra-pill ra-pill--${e}`, children: [
    r,
    "：",
    ln[e]
  ] });
}
function Us({ risks: r }) {
  return r == null || r.length === 0 ? /* @__PURE__ */ y(J, { label: "风险项" }) : /* @__PURE__ */ y("div", { className: "ra-risklist", children: r.map((e, t) => /* @__PURE__ */ $("div", { className: "ra-risk", children: [
    /* @__PURE__ */ $("div", { className: "ra-risk__head", children: [
      /* @__PURE__ */ y("span", { className: "ra-risk__name", children: c0(e.name, "风险名称") }),
      /* @__PURE__ */ $("span", { className: "ra-risk__levels", children: [
        e.impact ? /* @__PURE__ */ y(St, { prefix: "影响", level: e.impact }) : /* @__PURE__ */ y(J, { label: "影响程度" }),
        e.likelihood ? /* @__PURE__ */ y(St, { prefix: "可能性", level: e.likelihood }) : /* @__PURE__ */ y(J, { label: "发生可能性" })
      ] })
    ] }),
    /* @__PURE__ */ $("div", { className: "ra-risk__row", children: [
      /* @__PURE__ */ y("strong", { children: "缓解动作：" }),
      c0(e.mitigation, "缓解动作")
    ] }),
    /* @__PURE__ */ $("div", { className: "ra-risk__row", children: [
      /* @__PURE__ */ y("strong", { children: "负责人：" }),
      c0(e.owner, "负责人"),
      e.status ? /* @__PURE__ */ $(Ya, { children: [
        "　",
        /* @__PURE__ */ y("strong", { children: "状态：" }),
        e.status
      ] }) : null
    ] })
  ] }, t)) });
}
function js({
  question: r,
  options: e,
  criteria: t,
  recommended: a,
  rationale: n
}) {
  return /* @__PURE__ */ $("div", { className: "ra-decision", children: [
    /* @__PURE__ */ y("span", { className: "ra-decision__label", children: "决策" }),
    /* @__PURE__ */ $("div", { children: [
      /* @__PURE__ */ y("div", { className: "ra-decision__field-label", children: "问题" }),
      /* @__PURE__ */ y("div", { className: "ra-decision__question", children: c0(r, "决策问题") })
    ] }),
    /* @__PURE__ */ $("div", { children: [
      /* @__PURE__ */ y("div", { className: "ra-decision__field-label", children: "候选方案" }),
      e && e.length > 0 ? /* @__PURE__ */ y("ul", { className: "ra-decision__options", children: e.map((s, u) => /* @__PURE__ */ y("li", { children: s }, u)) }) : /* @__PURE__ */ y(J, { label: "候选方案" })
    ] }),
    t && t.length > 0 ? /* @__PURE__ */ $("div", { children: [
      /* @__PURE__ */ y("div", { className: "ra-decision__field-label", children: "判断标准" }),
      /* @__PURE__ */ y("ul", { className: "ra-decision__options", children: t.map((s, u) => /* @__PURE__ */ y("li", { children: s }, u)) })
    ] }) : null,
    /* @__PURE__ */ $("div", { className: "ra-decision__recommended", children: [
      /* @__PURE__ */ y("div", { className: "ra-decision__field-label", children: "推荐方案" }),
      /* @__PURE__ */ y("div", { children: c0(a, "推荐方案") }),
      /* @__PURE__ */ y("div", { className: "ra-decision__field-label ra-decision__field-label--spaced", children: "理由" }),
      /* @__PURE__ */ y("div", { children: c0(n, "推荐理由") })
    ] })
  ] });
}
function Xs({ items: r }) {
  return r == null || r.length === 0 ? /* @__PURE__ */ y(J, { label: "行动项" }) : /* @__PURE__ */ $("div", { className: "ra-actionlist", children: [
    /* @__PURE__ */ $("div", { className: "ra-actionlist__row ra-actionlist__head", children: [
      /* @__PURE__ */ y("span", { children: "行动项" }),
      /* @__PURE__ */ y("span", { children: "负责人" }),
      /* @__PURE__ */ y("span", { children: "截止" }),
      /* @__PURE__ */ y("span", { children: "状态" })
    ] }),
    r.map((e, t) => /* @__PURE__ */ $("div", { className: "ra-actionlist__row", children: [
      /* @__PURE__ */ y("span", { className: "ra-actionlist__task", children: c0(e.task, "行动项") }),
      /* @__PURE__ */ y("span", { children: c0(e.owner, "负责人") }),
      /* @__PURE__ */ y("span", { children: e.due ?? "—" }),
      /* @__PURE__ */ y("span", { children: e.status ?? "未开始" })
    ] }, t))
  ] });
}
function Vs({ question: r, options: e }) {
  return /* @__PURE__ */ $("div", { className: "ra-checkpoint", role: "note", children: [
    /* @__PURE__ */ y("div", { className: "ra-checkpoint__label", children: "需要人工确认 · Checkpoint" }),
    /* @__PURE__ */ y("div", { className: "ra-checkpoint__question", children: c0(r, "确认事项") }),
    e && e.length > 0 ? /* @__PURE__ */ y("div", { className: "ra-checkpoint__options", children: e.map((t, a) => /* @__PURE__ */ y("span", { className: "ra-checkpoint__option", children: t }, a)) }) : null
  ] });
}
function Ys({ subject: r, pros: e, cons: t, verdict: a }) {
  const n = e != null && e.length > 0, s = t != null && t.length > 0;
  return /* @__PURE__ */ $("div", { className: "ra-tradeoff", children: [
    /* @__PURE__ */ $("div", { className: "ra-tradeoff__head", children: [
      /* @__PURE__ */ y("span", { className: "ra-card__label", children: "权衡" }),
      r ? /* @__PURE__ */ y("span", { className: "ra-tradeoff__subject", children: r }) : null
    ] }),
    /* @__PURE__ */ $("div", { className: "ra-tradeoff__cols", children: [
      /* @__PURE__ */ $("div", { className: "ra-tradeoff__col", children: [
        /* @__PURE__ */ y("div", { className: "ra-tradeoff__col-label ra-tradeoff__col-label--pro", children: "利" }),
        n ? /* @__PURE__ */ y("ul", { className: "ra-tradeoff__list", children: e.map((u, o) => /* @__PURE__ */ y("li", { children: u }, o)) }) : /* @__PURE__ */ y(J, { label: "有利点" })
      ] }),
      /* @__PURE__ */ $("div", { className: "ra-tradeoff__col", children: [
        /* @__PURE__ */ y("div", { className: "ra-tradeoff__col-label ra-tradeoff__col-label--con", children: "弊" }),
        s ? /* @__PURE__ */ y("ul", { className: "ra-tradeoff__list", children: t.map((u, o) => /* @__PURE__ */ y("li", { children: u }, o)) }) : /* @__PURE__ */ y(J, { label: "不利点" })
      ] })
    ] }),
    a ? /* @__PURE__ */ $("div", { className: "ra-tradeoff__verdict", children: [
      /* @__PURE__ */ y("span", { className: "ra-tradeoff__verdict-label", children: "结论" }),
      a
    ] }) : null
  ] });
}
function Ws({
  title: r,
  severity: e,
  impact: t,
  rootCause: a,
  resolution: n,
  timeline: s,
  status: u
}) {
  return /* @__PURE__ */ $("div", { className: "ra-incident", children: [
    /* @__PURE__ */ $("div", { className: "ra-incident__head", children: [
      /* @__PURE__ */ y("span", { className: "ra-incident__label", children: "事件 · Incident" }),
      e ? /* @__PURE__ */ y(St, { prefix: "严重度", level: e }) : null,
      u ? /* @__PURE__ */ y("span", { className: "ra-incident__status", children: u }) : null
    ] }),
    /* @__PURE__ */ y("div", { className: "ra-incident__title", children: c0(r, "事件标题") }),
    /* @__PURE__ */ $("div", { className: "ra-incident__field", children: [
      /* @__PURE__ */ y("span", { className: "ra-incident__field-label", children: "影响" }),
      /* @__PURE__ */ y("span", { children: c0(t, "影响范围") })
    ] }),
    a ? /* @__PURE__ */ $("div", { className: "ra-incident__field", children: [
      /* @__PURE__ */ y("span", { className: "ra-incident__field-label", children: "根因" }),
      /* @__PURE__ */ y("span", { children: a })
    ] }) : null,
    n ? /* @__PURE__ */ $("div", { className: "ra-incident__field", children: [
      /* @__PURE__ */ y("span", { className: "ra-incident__field-label", children: "处置" }),
      /* @__PURE__ */ y("span", { children: n })
    ] }) : null,
    s && s.length > 0 ? /* @__PURE__ */ y("ul", { className: "ra-incident__timeline", children: s.map((o, h) => /* @__PURE__ */ $("li", { children: [
      /* @__PURE__ */ y("span", { className: "ra-incident__when", children: c0(o.when, "时间") }),
      /* @__PURE__ */ y("span", { children: c0(o.what, "动作") })
    ] }, h)) }) : null
  ] });
}
function un(r) {
  return r.length === 0 ? "(无行动项)" : r.map((e) => {
    const t = e.task ?? "未指定行动项", n = [`负责人: ${e.owner ?? "未指定负责人"}`];
    return e.due && n.push(`截止: ${e.due}`), e.status && n.push(`状态: ${e.status}`), `- [ ] ${t} (${n.join(", ")})`;
  }).join(`
`);
}
function on(r) {
  var t, a;
  const e = [];
  return e.push(`问题: ${r.question ?? "未指定问题"}`), (t = r.options) != null && t.length && e.push(`候选方案: ${r.options.join(" / ")}`), (a = r.criteria) != null && a.length && e.push(`判断标准: ${r.criteria.join(" / ")}`), e.push(`推荐方案: ${r.recommended ?? "未指定推荐方案"}`), e.push(`理由: ${r.rationale ?? "未指定理由"}`), e.join(`
`);
}
async function Yr(r) {
  try {
    if (navigator.clipboard && window.isSecureContext)
      return await navigator.clipboard.writeText(r), !0;
  } catch {
  }
  try {
    const e = document.createElement("textarea");
    e.value = r, e.style.position = "fixed", e.style.opacity = "0", document.body.appendChild(e), e.select();
    const t = document.execCommand("copy");
    return document.body.removeChild(e), t;
  } catch {
    return !1;
  }
}
var ur = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : typeof self < "u" ? self : {};
function cn(r) {
  return r && r.__esModule && Object.prototype.hasOwnProperty.call(r, "default") ? r.default : r;
}
var Wr = { exports: {} };
(function(r) {
  var e = typeof window < "u" ? window : typeof WorkerGlobalScope < "u" && self instanceof WorkerGlobalScope ? self : {};
  /**
   * Prism: Lightweight, robust, elegant syntax highlighting
   *
   * @license MIT <https://opensource.org/licenses/MIT>
   * @author Lea Verou <https://lea.verou.me>
   * @namespace
   * @public
   */
  var t = function(a) {
    var n = /(?:^|\s)lang(?:uage)?-([\w-]+)(?=\s|$)/i, s = 0, u = {}, o = {
      /**
       * By default, Prism will attempt to highlight all code elements (by calling {@link Prism.highlightAll}) on the
       * current page after the page finished loading. This might be a problem if e.g. you wanted to asynchronously load
       * additional languages or plugins yourself.
       *
       * By setting this value to `true`, Prism will not automatically highlight all code elements on the page.
       *
       * You obviously have to change this value before the automatic highlighting started. To do this, you can add an
       * empty Prism object into the global scope before loading the Prism script like this:
       *
       * ```js
       * window.Prism = window.Prism || {};
       * Prism.manual = true;
       * // add a new <script> to load Prism's script
       * ```
       *
       * @default false
       * @type {boolean}
       * @memberof Prism
       * @public
       */
      manual: a.Prism && a.Prism.manual,
      /**
       * By default, if Prism is in a web worker, it assumes that it is in a worker it created itself, so it uses
       * `addEventListener` to communicate with its parent instance. However, if you're using Prism manually in your
       * own worker, you don't want it to do this.
       *
       * By setting this value to `true`, Prism will not add its own listeners to the worker.
       *
       * You obviously have to change this value before Prism executes. To do this, you can add an
       * empty Prism object into the global scope before loading the Prism script like this:
       *
       * ```js
       * window.Prism = window.Prism || {};
       * Prism.disableWorkerMessageHandler = true;
       * // Load Prism's script
       * ```
       *
       * @default false
       * @type {boolean}
       * @memberof Prism
       * @public
       */
      disableWorkerMessageHandler: a.Prism && a.Prism.disableWorkerMessageHandler,
      /**
       * A namespace for utility methods.
       *
       * All function in this namespace that are not explicitly marked as _public_ are for __internal use only__ and may
       * change or disappear at any time.
       *
       * @namespace
       * @memberof Prism
       */
      util: {
        encode: function x(w) {
          return w instanceof h ? new h(w.type, x(w.content), w.alias) : Array.isArray(w) ? w.map(x) : w.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/\u00a0/g, " ");
        },
        /**
         * Returns the name of the type of the given value.
         *
         * @param {any} o
         * @returns {string}
         * @example
         * type(null)      === 'Null'
         * type(undefined) === 'Undefined'
         * type(123)       === 'Number'
         * type('foo')     === 'String'
         * type(true)      === 'Boolean'
         * type([1, 2])    === 'Array'
         * type({})        === 'Object'
         * type(String)    === 'Function'
         * type(/abc+/)    === 'RegExp'
         */
        type: function(x) {
          return Object.prototype.toString.call(x).slice(8, -1);
        },
        /**
         * Returns a unique number for the given object. Later calls will still return the same number.
         *
         * @param {Object} obj
         * @returns {number}
         */
        objId: function(x) {
          return x.__id || Object.defineProperty(x, "__id", { value: ++s }), x.__id;
        },
        /**
         * Creates a deep clone of the given object.
         *
         * The main intended use of this function is to clone language definitions.
         *
         * @param {T} o
         * @param {Record<number, any>} [visited]
         * @returns {T}
         * @template T
         */
        clone: function x(w, A) {
          A = A || {};
          var T, z;
          switch (o.util.type(w)) {
            case "Object":
              if (z = o.util.objId(w), A[z])
                return A[z];
              T = /** @type {Record<string, any>} */
              {}, A[z] = T;
              for (var q in w)
                w.hasOwnProperty(q) && (T[q] = x(w[q], A));
              return (
                /** @type {any} */
                T
              );
            case "Array":
              return z = o.util.objId(w), A[z] ? A[z] : (T = [], A[z] = T, /** @type {Array} */
              /** @type {any} */
              w.forEach(function(H, R) {
                T[R] = x(H, A);
              }), /** @type {any} */
              T);
            default:
              return w;
          }
        },
        /**
         * Returns the Prism language of the given element set by a `language-xxxx` or `lang-xxxx` class.
         *
         * If no language is set for the element or the element is `null` or `undefined`, `none` will be returned.
         *
         * @param {Element} element
         * @returns {string}
         */
        getLanguage: function(x) {
          for (; x; ) {
            var w = n.exec(x.className);
            if (w)
              return w[1].toLowerCase();
            x = x.parentElement;
          }
          return "none";
        },
        /**
         * Sets the Prism `language-xxxx` class of the given element.
         *
         * @param {Element} element
         * @param {string} language
         * @returns {void}
         */
        setLanguage: function(x, w) {
          x.className = x.className.replace(RegExp(n, "gi"), ""), x.classList.add("language-" + w);
        },
        /**
         * Returns the script element that is currently executing.
         *
         * This does __not__ work for line script element.
         *
         * @returns {HTMLScriptElement | null}
         */
        currentScript: function() {
          if (typeof document > "u")
            return null;
          if (document.currentScript && document.currentScript.tagName === "SCRIPT")
            return (
              /** @type {any} */
              document.currentScript
            );
          try {
            throw new Error();
          } catch (T) {
            var x = (/at [^(\r\n]*\((.*):[^:]+:[^:]+\)$/i.exec(T.stack) || [])[1];
            if (x) {
              var w = document.getElementsByTagName("script");
              for (var A in w)
                if (w[A].src == x)
                  return w[A];
            }
            return null;
          }
        },
        /**
         * Returns whether a given class is active for `element`.
         *
         * The class can be activated if `element` or one of its ancestors has the given class and it can be deactivated
         * if `element` or one of its ancestors has the negated version of the given class. The _negated version_ of the
         * given class is just the given class with a `no-` prefix.
         *
         * Whether the class is active is determined by the closest ancestor of `element` (where `element` itself is
         * closest ancestor) that has the given class or the negated version of it. If neither `element` nor any of its
         * ancestors have the given class or the negated version of it, then the default activation will be returned.
         *
         * In the paradoxical situation where the closest ancestor contains __both__ the given class and the negated
         * version of it, the class is considered active.
         *
         * @param {Element} element
         * @param {string} className
         * @param {boolean} [defaultActivation=false]
         * @returns {boolean}
         */
        isActive: function(x, w, A) {
          for (var T = "no-" + w; x; ) {
            var z = x.classList;
            if (z.contains(w))
              return !0;
            if (z.contains(T))
              return !1;
            x = x.parentElement;
          }
          return !!A;
        }
      },
      /**
       * This namespace contains all currently loaded languages and the some helper functions to create and modify languages.
       *
       * @namespace
       * @memberof Prism
       * @public
       */
      languages: {
        /**
         * The grammar for plain, unformatted text.
         */
        plain: u,
        plaintext: u,
        text: u,
        txt: u,
        /**
         * Creates a deep copy of the language with the given id and appends the given tokens.
         *
         * If a token in `redef` also appears in the copied language, then the existing token in the copied language
         * will be overwritten at its original position.
         *
         * ## Best practices
         *
         * Since the position of overwriting tokens (token in `redef` that overwrite tokens in the copied language)
         * doesn't matter, they can technically be in any order. However, this can be confusing to others that trying to
         * understand the language definition because, normally, the order of tokens matters in Prism grammars.
         *
         * Therefore, it is encouraged to order overwriting tokens according to the positions of the overwritten tokens.
         * Furthermore, all non-overwriting tokens should be placed after the overwriting ones.
         *
         * @param {string} id The id of the language to extend. This has to be a key in `Prism.languages`.
         * @param {Grammar} redef The new tokens to append.
         * @returns {Grammar} The new language created.
         * @public
         * @example
         * Prism.languages['css-with-colors'] = Prism.languages.extend('css', {
         *     // Prism.languages.css already has a 'comment' token, so this token will overwrite CSS' 'comment' token
         *     // at its original position
         *     'comment': { ... },
         *     // CSS doesn't have a 'color' token, so this token will be appended
         *     'color': /\b(?:red|green|blue)\b/
         * });
         */
        extend: function(x, w) {
          var A = o.util.clone(o.languages[x]);
          for (var T in w)
            A[T] = w[T];
          return A;
        },
        /**
         * Inserts tokens _before_ another token in a language definition or any other grammar.
         *
         * ## Usage
         *
         * This helper method makes it easy to modify existing languages. For example, the CSS language definition
         * not only defines CSS highlighting for CSS documents, but also needs to define highlighting for CSS embedded
         * in HTML through `<style>` elements. To do this, it needs to modify `Prism.languages.markup` and add the
         * appropriate tokens. However, `Prism.languages.markup` is a regular JavaScript object literal, so if you do
         * this:
         *
         * ```js
         * Prism.languages.markup.style = {
         *     // token
         * };
         * ```
         *
         * then the `style` token will be added (and processed) at the end. `insertBefore` allows you to insert tokens
         * before existing tokens. For the CSS example above, you would use it like this:
         *
         * ```js
         * Prism.languages.insertBefore('markup', 'cdata', {
         *     'style': {
         *         // token
         *     }
         * });
         * ```
         *
         * ## Special cases
         *
         * If the grammars of `inside` and `insert` have tokens with the same name, the tokens in `inside`'s grammar
         * will be ignored.
         *
         * This behavior can be used to insert tokens after `before`:
         *
         * ```js
         * Prism.languages.insertBefore('markup', 'comment', {
         *     'comment': Prism.languages.markup.comment,
         *     // tokens after 'comment'
         * });
         * ```
         *
         * ## Limitations
         *
         * The main problem `insertBefore` has to solve is iteration order. Since ES2015, the iteration order for object
         * properties is guaranteed to be the insertion order (except for integer keys) but some browsers behave
         * differently when keys are deleted and re-inserted. So `insertBefore` can't be implemented by temporarily
         * deleting properties which is necessary to insert at arbitrary positions.
         *
         * To solve this problem, `insertBefore` doesn't actually insert the given tokens into the target object.
         * Instead, it will create a new object and replace all references to the target object with the new one. This
         * can be done without temporarily deleting properties, so the iteration order is well-defined.
         *
         * However, only references that can be reached from `Prism.languages` or `insert` will be replaced. I.e. if
         * you hold the target object in a variable, then the value of the variable will not change.
         *
         * ```js
         * var oldMarkup = Prism.languages.markup;
         * var newMarkup = Prism.languages.insertBefore('markup', 'comment', { ... });
         *
         * assert(oldMarkup !== Prism.languages.markup);
         * assert(newMarkup === Prism.languages.markup);
         * ```
         *
         * @param {string} inside The property of `root` (e.g. a language id in `Prism.languages`) that contains the
         * object to be modified.
         * @param {string} before The key to insert before.
         * @param {Grammar} insert An object containing the key-value pairs to be inserted.
         * @param {Object<string, any>} [root] The object containing `inside`, i.e. the object that contains the
         * object to be modified.
         *
         * Defaults to `Prism.languages`.
         * @returns {Grammar} The new grammar object.
         * @public
         */
        insertBefore: function(x, w, A, T) {
          T = T || /** @type {any} */
          o.languages;
          var z = T[x], q = {};
          for (var H in z)
            if (z.hasOwnProperty(H)) {
              if (H == w)
                for (var R in A)
                  A.hasOwnProperty(R) && (q[R] = A[R]);
              A.hasOwnProperty(H) || (q[H] = z[H]);
            }
          var X = T[x];
          return T[x] = q, o.languages.DFS(o.languages, function(W, Z) {
            Z === X && W != x && (this[W] = q);
          }), q;
        },
        // Traverse a language definition with Depth First Search
        DFS: function x(w, A, T, z) {
          z = z || {};
          var q = o.util.objId;
          for (var H in w)
            if (w.hasOwnProperty(H)) {
              A.call(w, H, w[H], T || H);
              var R = w[H], X = o.util.type(R);
              X === "Object" && !z[q(R)] ? (z[q(R)] = !0, x(R, A, null, z)) : X === "Array" && !z[q(R)] && (z[q(R)] = !0, x(R, A, H, z));
            }
        }
      },
      plugins: {},
      /**
       * This is the most high-level function in Prism’s API.
       * It fetches all the elements that have a `.language-xxxx` class and then calls {@link Prism.highlightElement} on
       * each one of them.
       *
       * This is equivalent to `Prism.highlightAllUnder(document, async, callback)`.
       *
       * @param {boolean} [async=false] Same as in {@link Prism.highlightAllUnder}.
       * @param {HighlightCallback} [callback] Same as in {@link Prism.highlightAllUnder}.
       * @memberof Prism
       * @public
       */
      highlightAll: function(x, w) {
        o.highlightAllUnder(document, x, w);
      },
      /**
       * Fetches all the descendants of `container` that have a `.language-xxxx` class and then calls
       * {@link Prism.highlightElement} on each one of them.
       *
       * The following hooks will be run:
       * 1. `before-highlightall`
       * 2. `before-all-elements-highlight`
       * 3. All hooks of {@link Prism.highlightElement} for each element.
       *
       * @param {ParentNode} container The root element, whose descendants that have a `.language-xxxx` class will be highlighted.
       * @param {boolean} [async=false] Whether each element is to be highlighted asynchronously using Web Workers.
       * @param {HighlightCallback} [callback] An optional callback to be invoked on each element after its highlighting is done.
       * @memberof Prism
       * @public
       */
      highlightAllUnder: function(x, w, A) {
        var T = {
          callback: A,
          container: x,
          selector: 'code[class*="language-"], [class*="language-"] code, code[class*="lang-"], [class*="lang-"] code'
        };
        o.hooks.run("before-highlightall", T), T.elements = Array.prototype.slice.apply(T.container.querySelectorAll(T.selector)), o.hooks.run("before-all-elements-highlight", T);
        for (var z = 0, q; q = T.elements[z++]; )
          o.highlightElement(q, w === !0, T.callback);
      },
      /**
       * Highlights the code inside a single element.
       *
       * The following hooks will be run:
       * 1. `before-sanity-check`
       * 2. `before-highlight`
       * 3. All hooks of {@link Prism.highlight}. These hooks will be run by an asynchronous worker if `async` is `true`.
       * 4. `before-insert`
       * 5. `after-highlight`
       * 6. `complete`
       *
       * Some the above hooks will be skipped if the element doesn't contain any text or there is no grammar loaded for
       * the element's language.
       *
       * @param {Element} element The element containing the code.
       * It must have a class of `language-xxxx` to be processed, where `xxxx` is a valid language identifier.
       * @param {boolean} [async=false] Whether the element is to be highlighted asynchronously using Web Workers
       * to improve performance and avoid blocking the UI when highlighting very large chunks of code. This option is
       * [disabled by default](https://prismjs.com/faq.html#why-is-asynchronous-highlighting-disabled-by-default).
       *
       * Note: All language definitions required to highlight the code must be included in the main `prism.js` file for
       * asynchronous highlighting to work. You can build your own bundle on the
       * [Download page](https://prismjs.com/download.html).
       * @param {HighlightCallback} [callback] An optional callback to be invoked after the highlighting is done.
       * Mostly useful when `async` is `true`, since in that case, the highlighting is done asynchronously.
       * @memberof Prism
       * @public
       */
      highlightElement: function(x, w, A) {
        var T = o.util.getLanguage(x), z = o.languages[T];
        o.util.setLanguage(x, T);
        var q = x.parentElement;
        q && q.nodeName.toLowerCase() === "pre" && o.util.setLanguage(q, T);
        var H = x.textContent, R = {
          element: x,
          language: T,
          grammar: z,
          code: H
        };
        function X(Z) {
          R.highlightedCode = Z, o.hooks.run("before-insert", R), R.element.innerHTML = R.highlightedCode, o.hooks.run("after-highlight", R), o.hooks.run("complete", R), A && A.call(R.element);
        }
        if (o.hooks.run("before-sanity-check", R), q = R.element.parentElement, q && q.nodeName.toLowerCase() === "pre" && !q.hasAttribute("tabindex") && q.setAttribute("tabindex", "0"), !R.code) {
          o.hooks.run("complete", R), A && A.call(R.element);
          return;
        }
        if (o.hooks.run("before-highlight", R), !R.grammar) {
          X(o.util.encode(R.code));
          return;
        }
        if (w && a.Worker) {
          var W = new Worker(o.filename);
          W.onmessage = function(Z) {
            X(Z.data);
          }, W.postMessage(JSON.stringify({
            language: R.language,
            code: R.code,
            immediateClose: !0
          }));
        } else
          X(o.highlight(R.code, R.grammar, R.language));
      },
      /**
       * Low-level function, only use if you know what you’re doing. It accepts a string of text as input
       * and the language definitions to use, and returns a string with the HTML produced.
       *
       * The following hooks will be run:
       * 1. `before-tokenize`
       * 2. `after-tokenize`
       * 3. `wrap`: On each {@link Token}.
       *
       * @param {string} text A string with the code to be highlighted.
       * @param {Grammar} grammar An object containing the tokens to use.
       *
       * Usually a language definition like `Prism.languages.markup`.
       * @param {string} language The name of the language definition passed to `grammar`.
       * @returns {string} The highlighted HTML.
       * @memberof Prism
       * @public
       * @example
       * Prism.highlight('var foo = true;', Prism.languages.javascript, 'javascript');
       */
      highlight: function(x, w, A) {
        var T = {
          code: x,
          grammar: w,
          language: A
        };
        if (o.hooks.run("before-tokenize", T), !T.grammar)
          throw new Error('The language "' + T.language + '" has no grammar.');
        return T.tokens = o.tokenize(T.code, T.grammar), o.hooks.run("after-tokenize", T), h.stringify(o.util.encode(T.tokens), T.language);
      },
      /**
       * This is the heart of Prism, and the most low-level function you can use. It accepts a string of text as input
       * and the language definitions to use, and returns an array with the tokenized code.
       *
       * When the language definition includes nested tokens, the function is called recursively on each of these tokens.
       *
       * This method could be useful in other contexts as well, as a very crude parser.
       *
       * @param {string} text A string with the code to be highlighted.
       * @param {Grammar} grammar An object containing the tokens to use.
       *
       * Usually a language definition like `Prism.languages.markup`.
       * @returns {TokenStream} An array of strings and tokens, a token stream.
       * @memberof Prism
       * @public
       * @example
       * let code = `var foo = 0;`;
       * let tokens = Prism.tokenize(code, Prism.languages.javascript);
       * tokens.forEach(token => {
       *     if (token instanceof Prism.Token && token.type === 'number') {
       *         console.log(`Found numeric literal: ${token.content}`);
       *     }
       * });
       */
      tokenize: function(x, w) {
        var A = w.rest;
        if (A) {
          for (var T in A)
            w[T] = A[T];
          delete w.rest;
        }
        var z = new b();
        return k(z, z.head, x), v(x, z, w, z.head, 0), _(z);
      },
      /**
       * @namespace
       * @memberof Prism
       * @public
       */
      hooks: {
        all: {},
        /**
         * Adds the given callback to the list of callbacks for the given hook.
         *
         * The callback will be invoked when the hook it is registered for is run.
         * Hooks are usually directly run by a highlight function but you can also run hooks yourself.
         *
         * One callback function can be registered to multiple hooks and the same hook multiple times.
         *
         * @param {string} name The name of the hook.
         * @param {HookCallback} callback The callback function which is given environment variables.
         * @public
         */
        add: function(x, w) {
          var A = o.hooks.all;
          A[x] = A[x] || [], A[x].push(w);
        },
        /**
         * Runs a hook invoking all registered callbacks with the given environment variables.
         *
         * Callbacks will be invoked synchronously and in the order in which they were registered.
         *
         * @param {string} name The name of the hook.
         * @param {Object<string, any>} env The environment variables of the hook passed to all callbacks registered.
         * @public
         */
        run: function(x, w) {
          var A = o.hooks.all[x];
          if (!(!A || !A.length))
            for (var T = 0, z; z = A[T++]; )
              z(w);
        }
      },
      Token: h
    };
    a.Prism = o;
    function h(x, w, A, T) {
      this.type = x, this.content = w, this.alias = A, this.length = (T || "").length | 0;
    }
    h.stringify = function x(w, A) {
      if (typeof w == "string")
        return w;
      if (Array.isArray(w)) {
        var T = "";
        return w.forEach(function(X) {
          T += x(X, A);
        }), T;
      }
      var z = {
        type: w.type,
        content: x(w.content, A),
        tag: "span",
        classes: ["token", w.type],
        attributes: {},
        language: A
      }, q = w.alias;
      q && (Array.isArray(q) ? Array.prototype.push.apply(z.classes, q) : z.classes.push(q)), o.hooks.run("wrap", z);
      var H = "";
      for (var R in z.attributes)
        H += " " + R + '="' + (z.attributes[R] || "").replace(/"/g, "&quot;") + '"';
      return "<" + z.tag + ' class="' + z.classes.join(" ") + '"' + H + ">" + z.content + "</" + z.tag + ">";
    };
    function m(x, w, A, T) {
      x.lastIndex = w;
      var z = x.exec(A);
      if (z && T && z[1]) {
        var q = z[1].length;
        z.index += q, z[0] = z[0].slice(q);
      }
      return z;
    }
    function v(x, w, A, T, z, q) {
      for (var H in A)
        if (!(!A.hasOwnProperty(H) || !A[H])) {
          var R = A[H];
          R = Array.isArray(R) ? R : [R];
          for (var X = 0; X < R.length; ++X) {
            if (q && q.cause == H + "," + X)
              return;
            var W = R[X], Z = W.inside, V0 = !!W.lookbehind, x0 = !!W.greedy, w0 = W.alias;
            if (x0 && !W.pattern.global) {
              var D0 = W.pattern.toString().match(/[imsuy]*$/)[0];
              W.pattern = RegExp(W.pattern.source, D0 + "g");
            }
            for (var O0 = W.pattern || W, t0 = T.next, b0 = z; t0 !== w.tail && !(q && b0 >= q.reach); b0 += t0.value.length, t0 = t0.next) {
              var m0 = t0.value;
              if (w.length > x.length)
                return;
              if (!(m0 instanceof h)) {
                var S0 = 1, h0;
                if (x0) {
                  if (h0 = m(O0, b0, x, V0), !h0 || h0.index >= x.length)
                    break;
                  var oe = h0.index, k0 = h0.index + h0[0].length, A0 = b0;
                  for (A0 += t0.value.length; oe >= A0; )
                    t0 = t0.next, A0 += t0.value.length;
                  if (A0 -= t0.value.length, b0 = A0, t0.value instanceof h)
                    continue;
                  for (var Y0 = t0; Y0 !== w.tail && (A0 < k0 || typeof Y0.value == "string"); Y0 = Y0.next)
                    S0++, A0 += Y0.value.length;
                  S0--, m0 = x.slice(b0, A0), h0.index -= b0;
                } else if (h0 = m(O0, 0, m0, V0), !h0)
                  continue;
                var oe = h0.index, E0 = h0[0], ce = m0.slice(0, oe), L0 = m0.slice(oe + E0.length), he = b0 + m0.length;
                q && he > q.reach && (q.reach = he);
                var ne = t0.prev;
                ce && (ne = k(w, ne, ce), b0 += ce.length), S(w, ne, S0);
                var Fe = new h(H, Z ? o.tokenize(E0, Z) : E0, w0, E0);
                if (t0 = k(w, ne, Fe), L0 && k(w, t0, L0), S0 > 1) {
                  var W0 = {
                    cause: H + "," + X,
                    reach: he
                  };
                  v(x, w, A, t0.prev, b0, W0), q && W0.reach > q.reach && (q.reach = W0.reach);
                }
              }
            }
          }
        }
    }
    function b() {
      var x = { value: null, prev: null, next: null }, w = { value: null, prev: x, next: null };
      x.next = w, this.head = x, this.tail = w, this.length = 0;
    }
    function k(x, w, A) {
      var T = w.next, z = { value: A, prev: w, next: T };
      return w.next = z, T.prev = z, x.length++, z;
    }
    function S(x, w, A) {
      for (var T = w.next, z = 0; z < A && T !== x.tail; z++)
        T = T.next;
      w.next = T, T.prev = w, x.length -= z;
    }
    function _(x) {
      for (var w = [], A = x.head.next; A !== x.tail; )
        w.push(A.value), A = A.next;
      return w;
    }
    if (!a.document)
      return a.addEventListener && (o.disableWorkerMessageHandler || a.addEventListener("message", function(x) {
        var w = JSON.parse(x.data), A = w.language, T = w.code, z = w.immediateClose;
        a.postMessage(o.highlight(T, o.languages[A], A)), z && a.close();
      }, !1)), o;
    var C = o.util.currentScript();
    C && (o.filename = C.src, C.hasAttribute("data-manual") && (o.manual = !0));
    function N() {
      o.manual || o.highlightAll();
    }
    if (!o.manual) {
      var I = document.readyState;
      I === "loading" || I === "interactive" && C && C.defer ? document.addEventListener("DOMContentLoaded", N) : window.requestAnimationFrame ? window.requestAnimationFrame(N) : window.setTimeout(N, 16);
    }
    return o;
  }(e);
  r.exports && (r.exports = t), typeof ur < "u" && (ur.Prism = t), t.languages.markup = {
    comment: {
      pattern: /<!--(?:(?!<!--)[\s\S])*?-->/,
      greedy: !0
    },
    prolog: {
      pattern: /<\?[\s\S]+?\?>/,
      greedy: !0
    },
    doctype: {
      // https://www.w3.org/TR/xml/#NT-doctypedecl
      pattern: /<!DOCTYPE(?:[^>"'[\]]|"[^"]*"|'[^']*')+(?:\[(?:[^<"'\]]|"[^"]*"|'[^']*'|<(?!!--)|<!--(?:[^-]|-(?!->))*-->)*\]\s*)?>/i,
      greedy: !0,
      inside: {
        "internal-subset": {
          pattern: /(^[^\[]*\[)[\s\S]+(?=\]>$)/,
          lookbehind: !0,
          greedy: !0,
          inside: null
          // see below
        },
        string: {
          pattern: /"[^"]*"|'[^']*'/,
          greedy: !0
        },
        punctuation: /^<!|>$|[[\]]/,
        "doctype-tag": /^DOCTYPE/i,
        name: /[^\s<>'"]+/
      }
    },
    cdata: {
      pattern: /<!\[CDATA\[[\s\S]*?\]\]>/i,
      greedy: !0
    },
    tag: {
      pattern: /<\/?(?!\d)[^\s>\/=$<%]+(?:\s(?:\s*[^\s>\/=]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s'">=]+(?=[\s>]))|(?=[\s/>])))+)?\s*\/?>/,
      greedy: !0,
      inside: {
        tag: {
          pattern: /^<\/?[^\s>\/]+/,
          inside: {
            punctuation: /^<\/?/,
            namespace: /^[^\s>\/:]+:/
          }
        },
        "special-attr": [],
        "attr-value": {
          pattern: /=\s*(?:"[^"]*"|'[^']*'|[^\s'">=]+)/,
          inside: {
            punctuation: [
              {
                pattern: /^=/,
                alias: "attr-equals"
              },
              {
                pattern: /^(\s*)["']|["']$/,
                lookbehind: !0
              }
            ]
          }
        },
        punctuation: /\/?>/,
        "attr-name": {
          pattern: /[^\s>\/]+/,
          inside: {
            namespace: /^[^\s>\/:]+:/
          }
        }
      }
    },
    entity: [
      {
        pattern: /&[\da-z]{1,8};/i,
        alias: "named-entity"
      },
      /&#x?[\da-f]{1,8};/i
    ]
  }, t.languages.markup.tag.inside["attr-value"].inside.entity = t.languages.markup.entity, t.languages.markup.doctype.inside["internal-subset"].inside = t.languages.markup, t.hooks.add("wrap", function(a) {
    a.type === "entity" && (a.attributes.title = a.content.replace(/&amp;/, "&"));
  }), Object.defineProperty(t.languages.markup.tag, "addInlined", {
    /**
     * Adds an inlined language to markup.
     *
     * An example of an inlined language is CSS with `<style>` tags.
     *
     * @param {string} tagName The name of the tag that contains the inlined language. This name will be treated as
     * case insensitive.
     * @param {string} lang The language key.
     * @example
     * addInlined('style', 'css');
     */
    value: function(n, s) {
      var u = {};
      u["language-" + s] = {
        pattern: /(^<!\[CDATA\[)[\s\S]+?(?=\]\]>$)/i,
        lookbehind: !0,
        inside: t.languages[s]
      }, u.cdata = /^<!\[CDATA\[|\]\]>$/i;
      var o = {
        "included-cdata": {
          pattern: /<!\[CDATA\[[\s\S]*?\]\]>/i,
          inside: u
        }
      };
      o["language-" + s] = {
        pattern: /[\s\S]+/,
        inside: t.languages[s]
      };
      var h = {};
      h[n] = {
        pattern: RegExp(/(<__[^>]*>)(?:<!\[CDATA\[(?:[^\]]|\](?!\]>))*\]\]>|(?!<!\[CDATA\[)[\s\S])*?(?=<\/__>)/.source.replace(/__/g, function() {
          return n;
        }), "i"),
        lookbehind: !0,
        greedy: !0,
        inside: o
      }, t.languages.insertBefore("markup", "cdata", h);
    }
  }), Object.defineProperty(t.languages.markup.tag, "addAttribute", {
    /**
     * Adds an pattern to highlight languages embedded in HTML attributes.
     *
     * An example of an inlined language is CSS with `style` attributes.
     *
     * @param {string} attrName The name of the tag that contains the inlined language. This name will be treated as
     * case insensitive.
     * @param {string} lang The language key.
     * @example
     * addAttribute('style', 'css');
     */
    value: function(a, n) {
      t.languages.markup.tag.inside["special-attr"].push({
        pattern: RegExp(
          /(^|["'\s])/.source + "(?:" + a + ")" + /\s*=\s*(?:"[^"]*"|'[^']*'|[^\s'">=]+(?=[\s>]))/.source,
          "i"
        ),
        lookbehind: !0,
        inside: {
          "attr-name": /^[^\s=]+/,
          "attr-value": {
            pattern: /=[\s\S]+/,
            inside: {
              value: {
                pattern: /(^=\s*(["']|(?!["'])))\S[\s\S]*(?=\2$)/,
                lookbehind: !0,
                alias: [n, "language-" + n],
                inside: t.languages[n]
              },
              punctuation: [
                {
                  pattern: /^=/,
                  alias: "attr-equals"
                },
                /"|'/
              ]
            }
          }
        }
      });
    }
  }), t.languages.html = t.languages.markup, t.languages.mathml = t.languages.markup, t.languages.svg = t.languages.markup, t.languages.xml = t.languages.extend("markup", {}), t.languages.ssml = t.languages.xml, t.languages.atom = t.languages.xml, t.languages.rss = t.languages.xml, function(a) {
    var n = /(?:"(?:\\(?:\r\n|[\s\S])|[^"\\\r\n])*"|'(?:\\(?:\r\n|[\s\S])|[^'\\\r\n])*')/;
    a.languages.css = {
      comment: /\/\*[\s\S]*?\*\//,
      atrule: {
        pattern: RegExp("@[\\w-](?:" + /[^;{\s"']|\s+(?!\s)/.source + "|" + n.source + ")*?" + /(?:;|(?=\s*\{))/.source),
        inside: {
          rule: /^@[\w-]+/,
          "selector-function-argument": {
            pattern: /(\bselector\s*\(\s*(?![\s)]))(?:[^()\s]|\s+(?![\s)])|\((?:[^()]|\([^()]*\))*\))+(?=\s*\))/,
            lookbehind: !0,
            alias: "selector"
          },
          keyword: {
            pattern: /(^|[^\w-])(?:and|not|only|or)(?![\w-])/,
            lookbehind: !0
          }
          // See rest below
        }
      },
      url: {
        // https://drafts.csswg.org/css-values-3/#urls
        pattern: RegExp("\\burl\\((?:" + n.source + "|" + /(?:[^\\\r\n()"']|\\[\s\S])*/.source + ")\\)", "i"),
        greedy: !0,
        inside: {
          function: /^url/i,
          punctuation: /^\(|\)$/,
          string: {
            pattern: RegExp("^" + n.source + "$"),
            alias: "url"
          }
        }
      },
      selector: {
        pattern: RegExp(`(^|[{}\\s])[^{}\\s](?:[^{};"'\\s]|\\s+(?![\\s{])|` + n.source + ")*(?=\\s*\\{)"),
        lookbehind: !0
      },
      string: {
        pattern: n,
        greedy: !0
      },
      property: {
        pattern: /(^|[^-\w\xA0-\uFFFF])(?!\s)[-_a-z\xA0-\uFFFF](?:(?!\s)[-\w\xA0-\uFFFF])*(?=\s*:)/i,
        lookbehind: !0
      },
      important: /!important\b/i,
      function: {
        pattern: /(^|[^-a-z0-9])[-a-z0-9]+(?=\()/i,
        lookbehind: !0
      },
      punctuation: /[(){};:,]/
    }, a.languages.css.atrule.inside.rest = a.languages.css;
    var s = a.languages.markup;
    s && (s.tag.addInlined("style", "css"), s.tag.addAttribute("style", "css"));
  }(t), t.languages.clike = {
    comment: [
      {
        pattern: /(^|[^\\])\/\*[\s\S]*?(?:\*\/|$)/,
        lookbehind: !0,
        greedy: !0
      },
      {
        pattern: /(^|[^\\:])\/\/.*/,
        lookbehind: !0,
        greedy: !0
      }
    ],
    string: {
      pattern: /(["'])(?:\\(?:\r\n|[\s\S])|(?!\1)[^\\\r\n])*\1/,
      greedy: !0
    },
    "class-name": {
      pattern: /(\b(?:class|extends|implements|instanceof|interface|new|trait)\s+|\bcatch\s+\()[\w.\\]+/i,
      lookbehind: !0,
      inside: {
        punctuation: /[.\\]/
      }
    },
    keyword: /\b(?:break|catch|continue|do|else|finally|for|function|if|in|instanceof|new|null|return|throw|try|while)\b/,
    boolean: /\b(?:false|true)\b/,
    function: /\b\w+(?=\()/,
    number: /\b0x[\da-f]+\b|(?:\b\d+(?:\.\d*)?|\B\.\d+)(?:e[+-]?\d+)?/i,
    operator: /[<>]=?|[!=]=?=?|--?|\+\+?|&&?|\|\|?|[?*/~^%]/,
    punctuation: /[{}[\];(),.:]/
  }, t.languages.javascript = t.languages.extend("clike", {
    "class-name": [
      t.languages.clike["class-name"],
      {
        pattern: /(^|[^$\w\xA0-\uFFFF])(?!\s)[_$A-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\.(?:constructor|prototype))/,
        lookbehind: !0
      }
    ],
    keyword: [
      {
        pattern: /((?:^|\})\s*)catch\b/,
        lookbehind: !0
      },
      {
        pattern: /(^|[^.]|\.\.\.\s*)\b(?:as|assert(?=\s*\{)|async(?=\s*(?:function\b|\(|[$\w\xA0-\uFFFF]|$))|await|break|case|class|const|continue|debugger|default|delete|do|else|enum|export|extends|finally(?=\s*(?:\{|$))|for|from(?=\s*(?:['"]|$))|function|(?:get|set)(?=\s*(?:[#\[$\w\xA0-\uFFFF]|$))|if|implements|import|in|instanceof|interface|let|new|null|of|package|private|protected|public|return|static|super|switch|this|throw|try|typeof|undefined|var|void|while|with|yield)\b/,
        lookbehind: !0
      }
    ],
    // Allow for all non-ASCII characters (See http://stackoverflow.com/a/2008444)
    function: /#?(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\s*(?:\.\s*(?:apply|bind|call)\s*)?\()/,
    number: {
      pattern: RegExp(
        /(^|[^\w$])/.source + "(?:" + // constant
        (/NaN|Infinity/.source + "|" + // binary integer
        /0[bB][01]+(?:_[01]+)*n?/.source + "|" + // octal integer
        /0[oO][0-7]+(?:_[0-7]+)*n?/.source + "|" + // hexadecimal integer
        /0[xX][\dA-Fa-f]+(?:_[\dA-Fa-f]+)*n?/.source + "|" + // decimal bigint
        /\d+(?:_\d+)*n/.source + "|" + // decimal number (integer or float) but no bigint
        /(?:\d+(?:_\d+)*(?:\.(?:\d+(?:_\d+)*)?)?|\.\d+(?:_\d+)*)(?:[Ee][+-]?\d+(?:_\d+)*)?/.source) + ")" + /(?![\w$])/.source
      ),
      lookbehind: !0
    },
    operator: /--|\+\+|\*\*=?|=>|&&=?|\|\|=?|[!=]==|<<=?|>>>?=?|[-+*/%&|^!=<>]=?|\.{3}|\?\?=?|\?\.?|[~:]/
  }), t.languages.javascript["class-name"][0].pattern = /(\b(?:class|extends|implements|instanceof|interface|new)\s+)[\w.\\]+/, t.languages.insertBefore("javascript", "keyword", {
    regex: {
      pattern: RegExp(
        // lookbehind
        // eslint-disable-next-line regexp/no-dupe-characters-character-class
        /((?:^|[^$\w\xA0-\uFFFF."'\])\s]|\b(?:return|yield))\s*)/.source + // Regex pattern:
        // There are 2 regex patterns here. The RegExp set notation proposal added support for nested character
        // classes if the `v` flag is present. Unfortunately, nested CCs are both context-free and incompatible
        // with the only syntax, so we have to define 2 different regex patterns.
        /\//.source + "(?:" + /(?:\[(?:[^\]\\\r\n]|\\.)*\]|\\.|[^/\\\[\r\n])+\/[dgimyus]{0,7}/.source + "|" + // `v` flag syntax. This supports 3 levels of nested character classes.
        /(?:\[(?:[^[\]\\\r\n]|\\.|\[(?:[^[\]\\\r\n]|\\.|\[(?:[^[\]\\\r\n]|\\.)*\])*\])*\]|\\.|[^/\\\[\r\n])+\/[dgimyus]{0,7}v[dgimyus]{0,7}/.source + ")" + // lookahead
        /(?=(?:\s|\/\*(?:[^*]|\*(?!\/))*\*\/)*(?:$|[\r\n,.;:})\]]|\/\/))/.source
      ),
      lookbehind: !0,
      greedy: !0,
      inside: {
        "regex-source": {
          pattern: /^(\/)[\s\S]+(?=\/[a-z]*$)/,
          lookbehind: !0,
          alias: "language-regex",
          inside: t.languages.regex
        },
        "regex-delimiter": /^\/|\/$/,
        "regex-flags": /^[a-z]+$/
      }
    },
    // This must be declared before keyword because we use "function" inside the look-forward
    "function-variable": {
      pattern: /#?(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\s*[=:]\s*(?:async\s*)?(?:\bfunction\b|(?:\((?:[^()]|\([^()]*\))*\)|(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*)\s*=>))/,
      alias: "function"
    },
    parameter: [
      {
        pattern: /(function(?:\s+(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*)?\s*\(\s*)(?!\s)(?:[^()\s]|\s+(?![\s)])|\([^()]*\))+(?=\s*\))/,
        lookbehind: !0,
        inside: t.languages.javascript
      },
      {
        pattern: /(^|[^$\w\xA0-\uFFFF])(?!\s)[_$a-z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\s*=>)/i,
        lookbehind: !0,
        inside: t.languages.javascript
      },
      {
        pattern: /(\(\s*)(?!\s)(?:[^()\s]|\s+(?![\s)])|\([^()]*\))+(?=\s*\)\s*=>)/,
        lookbehind: !0,
        inside: t.languages.javascript
      },
      {
        pattern: /((?:\b|\s|^)(?!(?:as|async|await|break|case|catch|class|const|continue|debugger|default|delete|do|else|enum|export|extends|finally|for|from|function|get|if|implements|import|in|instanceof|interface|let|new|null|of|package|private|protected|public|return|set|static|super|switch|this|throw|try|typeof|undefined|var|void|while|with|yield)(?![$\w\xA0-\uFFFF]))(?:(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*\s*)\(\s*|\]\s*\(\s*)(?!\s)(?:[^()\s]|\s+(?![\s)])|\([^()]*\))+(?=\s*\)\s*\{)/,
        lookbehind: !0,
        inside: t.languages.javascript
      }
    ],
    constant: /\b[A-Z](?:[A-Z_]|\dx?)*\b/
  }), t.languages.insertBefore("javascript", "string", {
    hashbang: {
      pattern: /^#!.*/,
      greedy: !0,
      alias: "comment"
    },
    "template-string": {
      pattern: /`(?:\\[\s\S]|\$\{(?:[^{}]|\{(?:[^{}]|\{[^}]*\})*\})+\}|(?!\$\{)[^\\`])*`/,
      greedy: !0,
      inside: {
        "template-punctuation": {
          pattern: /^`|`$/,
          alias: "string"
        },
        interpolation: {
          pattern: /((?:^|[^\\])(?:\\{2})*)\$\{(?:[^{}]|\{(?:[^{}]|\{[^}]*\})*\})+\}/,
          lookbehind: !0,
          inside: {
            "interpolation-punctuation": {
              pattern: /^\$\{|\}$/,
              alias: "punctuation"
            },
            rest: t.languages.javascript
          }
        },
        string: /[\s\S]+/
      }
    },
    "string-property": {
      pattern: /((?:^|[,{])[ \t]*)(["'])(?:\\(?:\r\n|[\s\S])|(?!\2)[^\\\r\n])*\2(?=\s*:)/m,
      lookbehind: !0,
      greedy: !0,
      alias: "property"
    }
  }), t.languages.insertBefore("javascript", "operator", {
    "literal-property": {
      pattern: /((?:^|[,{])[ \t]*)(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\s*:)/m,
      lookbehind: !0,
      alias: "property"
    }
  }), t.languages.markup && (t.languages.markup.tag.addInlined("script", "javascript"), t.languages.markup.tag.addAttribute(
    /on(?:abort|blur|change|click|composition(?:end|start|update)|dblclick|error|focus(?:in|out)?|key(?:down|up)|load|mouse(?:down|enter|leave|move|out|over|up)|reset|resize|scroll|select|slotchange|submit|unload|wheel)/.source,
    "javascript"
  )), t.languages.js = t.languages.javascript, function() {
    if (typeof t > "u" || typeof document > "u")
      return;
    Element.prototype.matches || (Element.prototype.matches = Element.prototype.msMatchesSelector || Element.prototype.webkitMatchesSelector);
    var a = "Loading…", n = function(C, N) {
      return "✖ Error " + C + " while fetching file: " + N;
    }, s = "✖ Error: File does not exist or is empty", u = {
      js: "javascript",
      py: "python",
      rb: "ruby",
      ps1: "powershell",
      psm1: "powershell",
      sh: "bash",
      bat: "batch",
      h: "c",
      tex: "latex"
    }, o = "data-src-status", h = "loading", m = "loaded", v = "failed", b = "pre[data-src]:not([" + o + '="' + m + '"]):not([' + o + '="' + h + '"])';
    function k(C, N, I) {
      var x = new XMLHttpRequest();
      x.open("GET", C, !0), x.onreadystatechange = function() {
        x.readyState == 4 && (x.status < 400 && x.responseText ? N(x.responseText) : x.status >= 400 ? I(n(x.status, x.statusText)) : I(s));
      }, x.send(null);
    }
    function S(C) {
      var N = /^\s*(\d+)\s*(?:(,)\s*(?:(\d+)\s*)?)?$/.exec(C || "");
      if (N) {
        var I = Number(N[1]), x = N[2], w = N[3];
        return x ? w ? [I, Number(w)] : [I, void 0] : [I, I];
      }
    }
    t.hooks.add("before-highlightall", function(C) {
      C.selector += ", " + b;
    }), t.hooks.add("before-sanity-check", function(C) {
      var N = (
        /** @type {HTMLPreElement} */
        C.element
      );
      if (N.matches(b)) {
        C.code = "", N.setAttribute(o, h);
        var I = N.appendChild(document.createElement("CODE"));
        I.textContent = a;
        var x = N.getAttribute("data-src"), w = C.language;
        if (w === "none") {
          var A = (/\.(\w+)$/.exec(x) || [, "none"])[1];
          w = u[A] || A;
        }
        t.util.setLanguage(I, w), t.util.setLanguage(N, w);
        var T = t.plugins.autoloader;
        T && T.loadLanguages(w), k(
          x,
          function(z) {
            N.setAttribute(o, m);
            var q = S(N.getAttribute("data-range"));
            if (q) {
              var H = z.split(/\r\n?|\n/g), R = q[0], X = q[1] == null ? H.length : q[1];
              R < 0 && (R += H.length), R = Math.max(0, Math.min(R - 1, H.length)), X < 0 && (X += H.length), X = Math.max(0, Math.min(X, H.length)), z = H.slice(R, X).join(`
`), N.hasAttribute("data-start") || N.setAttribute("data-start", String(R + 1));
            }
            I.textContent = z, t.highlightElement(I);
          },
          function(z) {
            N.setAttribute(o, v), I.textContent = z;
          }
        );
      }
    }), t.plugins.fileHighlight = {
      /**
       * Executes the File Highlight plugin for all matching `pre` elements under the given container.
       *
       * Note: Elements which are already loaded or currently loading will not be touched by this method.
       *
       * @param {ParentNode} [container=document]
       */
      highlight: function(N) {
        for (var I = (N || document).querySelectorAll(b), x = 0, w; w = I[x++]; )
          t.highlightElement(w);
      }
    };
    var _ = !1;
    t.fileHighlight = function() {
      _ || (console.warn("Prism.fileHighlight is deprecated. Use `Prism.plugins.fileHighlight.highlight` instead."), _ = !0), t.plugins.fileHighlight.highlight.apply(this, arguments);
    };
  }();
})(Wr);
var hn = Wr.exports;
const nt = /* @__PURE__ */ cn(hn);
Prism.languages.markup = {
  comment: {
    pattern: /<!--(?:(?!<!--)[\s\S])*?-->/,
    greedy: !0
  },
  prolog: {
    pattern: /<\?[\s\S]+?\?>/,
    greedy: !0
  },
  doctype: {
    // https://www.w3.org/TR/xml/#NT-doctypedecl
    pattern: /<!DOCTYPE(?:[^>"'[\]]|"[^"]*"|'[^']*')+(?:\[(?:[^<"'\]]|"[^"]*"|'[^']*'|<(?!!--)|<!--(?:[^-]|-(?!->))*-->)*\]\s*)?>/i,
    greedy: !0,
    inside: {
      "internal-subset": {
        pattern: /(^[^\[]*\[)[\s\S]+(?=\]>$)/,
        lookbehind: !0,
        greedy: !0,
        inside: null
        // see below
      },
      string: {
        pattern: /"[^"]*"|'[^']*'/,
        greedy: !0
      },
      punctuation: /^<!|>$|[[\]]/,
      "doctype-tag": /^DOCTYPE/i,
      name: /[^\s<>'"]+/
    }
  },
  cdata: {
    pattern: /<!\[CDATA\[[\s\S]*?\]\]>/i,
    greedy: !0
  },
  tag: {
    pattern: /<\/?(?!\d)[^\s>\/=$<%]+(?:\s(?:\s*[^\s>\/=]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s'">=]+(?=[\s>]))|(?=[\s/>])))+)?\s*\/?>/,
    greedy: !0,
    inside: {
      tag: {
        pattern: /^<\/?[^\s>\/]+/,
        inside: {
          punctuation: /^<\/?/,
          namespace: /^[^\s>\/:]+:/
        }
      },
      "special-attr": [],
      "attr-value": {
        pattern: /=\s*(?:"[^"]*"|'[^']*'|[^\s'">=]+)/,
        inside: {
          punctuation: [
            {
              pattern: /^=/,
              alias: "attr-equals"
            },
            {
              pattern: /^(\s*)["']|["']$/,
              lookbehind: !0
            }
          ]
        }
      },
      punctuation: /\/?>/,
      "attr-name": {
        pattern: /[^\s>\/]+/,
        inside: {
          namespace: /^[^\s>\/:]+:/
        }
      }
    }
  },
  entity: [
    {
      pattern: /&[\da-z]{1,8};/i,
      alias: "named-entity"
    },
    /&#x?[\da-f]{1,8};/i
  ]
};
Prism.languages.markup.tag.inside["attr-value"].inside.entity = Prism.languages.markup.entity;
Prism.languages.markup.doctype.inside["internal-subset"].inside = Prism.languages.markup;
Prism.hooks.add("wrap", function(r) {
  r.type === "entity" && (r.attributes.title = r.content.replace(/&amp;/, "&"));
});
Object.defineProperty(Prism.languages.markup.tag, "addInlined", {
  /**
   * Adds an inlined language to markup.
   *
   * An example of an inlined language is CSS with `<style>` tags.
   *
   * @param {string} tagName The name of the tag that contains the inlined language. This name will be treated as
   * case insensitive.
   * @param {string} lang The language key.
   * @example
   * addInlined('style', 'css');
   */
  value: function(e, t) {
    var a = {};
    a["language-" + t] = {
      pattern: /(^<!\[CDATA\[)[\s\S]+?(?=\]\]>$)/i,
      lookbehind: !0,
      inside: Prism.languages[t]
    }, a.cdata = /^<!\[CDATA\[|\]\]>$/i;
    var n = {
      "included-cdata": {
        pattern: /<!\[CDATA\[[\s\S]*?\]\]>/i,
        inside: a
      }
    };
    n["language-" + t] = {
      pattern: /[\s\S]+/,
      inside: Prism.languages[t]
    };
    var s = {};
    s[e] = {
      pattern: RegExp(/(<__[^>]*>)(?:<!\[CDATA\[(?:[^\]]|\](?!\]>))*\]\]>|(?!<!\[CDATA\[)[\s\S])*?(?=<\/__>)/.source.replace(/__/g, function() {
        return e;
      }), "i"),
      lookbehind: !0,
      greedy: !0,
      inside: n
    }, Prism.languages.insertBefore("markup", "cdata", s);
  }
});
Object.defineProperty(Prism.languages.markup.tag, "addAttribute", {
  /**
   * Adds an pattern to highlight languages embedded in HTML attributes.
   *
   * An example of an inlined language is CSS with `style` attributes.
   *
   * @param {string} attrName The name of the tag that contains the inlined language. This name will be treated as
   * case insensitive.
   * @param {string} lang The language key.
   * @example
   * addAttribute('style', 'css');
   */
  value: function(r, e) {
    Prism.languages.markup.tag.inside["special-attr"].push({
      pattern: RegExp(
        /(^|["'\s])/.source + "(?:" + r + ")" + /\s*=\s*(?:"[^"]*"|'[^']*'|[^\s'">=]+(?=[\s>]))/.source,
        "i"
      ),
      lookbehind: !0,
      inside: {
        "attr-name": /^[^\s=]+/,
        "attr-value": {
          pattern: /=[\s\S]+/,
          inside: {
            value: {
              pattern: /(^=\s*(["']|(?!["'])))\S[\s\S]*(?=\2$)/,
              lookbehind: !0,
              alias: [e, "language-" + e],
              inside: Prism.languages[e]
            },
            punctuation: [
              {
                pattern: /^=/,
                alias: "attr-equals"
              },
              /"|'/
            ]
          }
        }
      }
    });
  }
});
Prism.languages.html = Prism.languages.markup;
Prism.languages.mathml = Prism.languages.markup;
Prism.languages.svg = Prism.languages.markup;
Prism.languages.xml = Prism.languages.extend("markup", {});
Prism.languages.ssml = Prism.languages.xml;
Prism.languages.atom = Prism.languages.xml;
Prism.languages.rss = Prism.languages.xml;
(function(r) {
  var e = /(?:"(?:\\(?:\r\n|[\s\S])|[^"\\\r\n])*"|'(?:\\(?:\r\n|[\s\S])|[^'\\\r\n])*')/;
  r.languages.css = {
    comment: /\/\*[\s\S]*?\*\//,
    atrule: {
      pattern: RegExp("@[\\w-](?:" + /[^;{\s"']|\s+(?!\s)/.source + "|" + e.source + ")*?" + /(?:;|(?=\s*\{))/.source),
      inside: {
        rule: /^@[\w-]+/,
        "selector-function-argument": {
          pattern: /(\bselector\s*\(\s*(?![\s)]))(?:[^()\s]|\s+(?![\s)])|\((?:[^()]|\([^()]*\))*\))+(?=\s*\))/,
          lookbehind: !0,
          alias: "selector"
        },
        keyword: {
          pattern: /(^|[^\w-])(?:and|not|only|or)(?![\w-])/,
          lookbehind: !0
        }
        // See rest below
      }
    },
    url: {
      // https://drafts.csswg.org/css-values-3/#urls
      pattern: RegExp("\\burl\\((?:" + e.source + "|" + /(?:[^\\\r\n()"']|\\[\s\S])*/.source + ")\\)", "i"),
      greedy: !0,
      inside: {
        function: /^url/i,
        punctuation: /^\(|\)$/,
        string: {
          pattern: RegExp("^" + e.source + "$"),
          alias: "url"
        }
      }
    },
    selector: {
      pattern: RegExp(`(^|[{}\\s])[^{}\\s](?:[^{};"'\\s]|\\s+(?![\\s{])|` + e.source + ")*(?=\\s*\\{)"),
      lookbehind: !0
    },
    string: {
      pattern: e,
      greedy: !0
    },
    property: {
      pattern: /(^|[^-\w\xA0-\uFFFF])(?!\s)[-_a-z\xA0-\uFFFF](?:(?!\s)[-\w\xA0-\uFFFF])*(?=\s*:)/i,
      lookbehind: !0
    },
    important: /!important\b/i,
    function: {
      pattern: /(^|[^-a-z0-9])[-a-z0-9]+(?=\()/i,
      lookbehind: !0
    },
    punctuation: /[(){};:,]/
  }, r.languages.css.atrule.inside.rest = r.languages.css;
  var t = r.languages.markup;
  t && (t.tag.addInlined("style", "css"), t.tag.addAttribute("style", "css"));
})(Prism);
Prism.languages.clike = {
  comment: [
    {
      pattern: /(^|[^\\])\/\*[\s\S]*?(?:\*\/|$)/,
      lookbehind: !0,
      greedy: !0
    },
    {
      pattern: /(^|[^\\:])\/\/.*/,
      lookbehind: !0,
      greedy: !0
    }
  ],
  string: {
    pattern: /(["'])(?:\\(?:\r\n|[\s\S])|(?!\1)[^\\\r\n])*\1/,
    greedy: !0
  },
  "class-name": {
    pattern: /(\b(?:class|extends|implements|instanceof|interface|new|trait)\s+|\bcatch\s+\()[\w.\\]+/i,
    lookbehind: !0,
    inside: {
      punctuation: /[.\\]/
    }
  },
  keyword: /\b(?:break|catch|continue|do|else|finally|for|function|if|in|instanceof|new|null|return|throw|try|while)\b/,
  boolean: /\b(?:false|true)\b/,
  function: /\b\w+(?=\()/,
  number: /\b0x[\da-f]+\b|(?:\b\d+(?:\.\d*)?|\B\.\d+)(?:e[+-]?\d+)?/i,
  operator: /[<>]=?|[!=]=?=?|--?|\+\+?|&&?|\|\|?|[?*/~^%]/,
  punctuation: /[{}[\];(),.:]/
};
Prism.languages.javascript = Prism.languages.extend("clike", {
  "class-name": [
    Prism.languages.clike["class-name"],
    {
      pattern: /(^|[^$\w\xA0-\uFFFF])(?!\s)[_$A-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\.(?:constructor|prototype))/,
      lookbehind: !0
    }
  ],
  keyword: [
    {
      pattern: /((?:^|\})\s*)catch\b/,
      lookbehind: !0
    },
    {
      pattern: /(^|[^.]|\.\.\.\s*)\b(?:as|assert(?=\s*\{)|async(?=\s*(?:function\b|\(|[$\w\xA0-\uFFFF]|$))|await|break|case|class|const|continue|debugger|default|delete|do|else|enum|export|extends|finally(?=\s*(?:\{|$))|for|from(?=\s*(?:['"]|$))|function|(?:get|set)(?=\s*(?:[#\[$\w\xA0-\uFFFF]|$))|if|implements|import|in|instanceof|interface|let|new|null|of|package|private|protected|public|return|static|super|switch|this|throw|try|typeof|undefined|var|void|while|with|yield)\b/,
      lookbehind: !0
    }
  ],
  // Allow for all non-ASCII characters (See http://stackoverflow.com/a/2008444)
  function: /#?(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\s*(?:\.\s*(?:apply|bind|call)\s*)?\()/,
  number: {
    pattern: RegExp(
      /(^|[^\w$])/.source + "(?:" + // constant
      (/NaN|Infinity/.source + "|" + // binary integer
      /0[bB][01]+(?:_[01]+)*n?/.source + "|" + // octal integer
      /0[oO][0-7]+(?:_[0-7]+)*n?/.source + "|" + // hexadecimal integer
      /0[xX][\dA-Fa-f]+(?:_[\dA-Fa-f]+)*n?/.source + "|" + // decimal bigint
      /\d+(?:_\d+)*n/.source + "|" + // decimal number (integer or float) but no bigint
      /(?:\d+(?:_\d+)*(?:\.(?:\d+(?:_\d+)*)?)?|\.\d+(?:_\d+)*)(?:[Ee][+-]?\d+(?:_\d+)*)?/.source) + ")" + /(?![\w$])/.source
    ),
    lookbehind: !0
  },
  operator: /--|\+\+|\*\*=?|=>|&&=?|\|\|=?|[!=]==|<<=?|>>>?=?|[-+*/%&|^!=<>]=?|\.{3}|\?\?=?|\?\.?|[~:]/
});
Prism.languages.javascript["class-name"][0].pattern = /(\b(?:class|extends|implements|instanceof|interface|new)\s+)[\w.\\]+/;
Prism.languages.insertBefore("javascript", "keyword", {
  regex: {
    pattern: RegExp(
      // lookbehind
      // eslint-disable-next-line regexp/no-dupe-characters-character-class
      /((?:^|[^$\w\xA0-\uFFFF."'\])\s]|\b(?:return|yield))\s*)/.source + // Regex pattern:
      // There are 2 regex patterns here. The RegExp set notation proposal added support for nested character
      // classes if the `v` flag is present. Unfortunately, nested CCs are both context-free and incompatible
      // with the only syntax, so we have to define 2 different regex patterns.
      /\//.source + "(?:" + /(?:\[(?:[^\]\\\r\n]|\\.)*\]|\\.|[^/\\\[\r\n])+\/[dgimyus]{0,7}/.source + "|" + // `v` flag syntax. This supports 3 levels of nested character classes.
      /(?:\[(?:[^[\]\\\r\n]|\\.|\[(?:[^[\]\\\r\n]|\\.|\[(?:[^[\]\\\r\n]|\\.)*\])*\])*\]|\\.|[^/\\\[\r\n])+\/[dgimyus]{0,7}v[dgimyus]{0,7}/.source + ")" + // lookahead
      /(?=(?:\s|\/\*(?:[^*]|\*(?!\/))*\*\/)*(?:$|[\r\n,.;:})\]]|\/\/))/.source
    ),
    lookbehind: !0,
    greedy: !0,
    inside: {
      "regex-source": {
        pattern: /^(\/)[\s\S]+(?=\/[a-z]*$)/,
        lookbehind: !0,
        alias: "language-regex",
        inside: Prism.languages.regex
      },
      "regex-delimiter": /^\/|\/$/,
      "regex-flags": /^[a-z]+$/
    }
  },
  // This must be declared before keyword because we use "function" inside the look-forward
  "function-variable": {
    pattern: /#?(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\s*[=:]\s*(?:async\s*)?(?:\bfunction\b|(?:\((?:[^()]|\([^()]*\))*\)|(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*)\s*=>))/,
    alias: "function"
  },
  parameter: [
    {
      pattern: /(function(?:\s+(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*)?\s*\(\s*)(?!\s)(?:[^()\s]|\s+(?![\s)])|\([^()]*\))+(?=\s*\))/,
      lookbehind: !0,
      inside: Prism.languages.javascript
    },
    {
      pattern: /(^|[^$\w\xA0-\uFFFF])(?!\s)[_$a-z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\s*=>)/i,
      lookbehind: !0,
      inside: Prism.languages.javascript
    },
    {
      pattern: /(\(\s*)(?!\s)(?:[^()\s]|\s+(?![\s)])|\([^()]*\))+(?=\s*\)\s*=>)/,
      lookbehind: !0,
      inside: Prism.languages.javascript
    },
    {
      pattern: /((?:\b|\s|^)(?!(?:as|async|await|break|case|catch|class|const|continue|debugger|default|delete|do|else|enum|export|extends|finally|for|from|function|get|if|implements|import|in|instanceof|interface|let|new|null|of|package|private|protected|public|return|set|static|super|switch|this|throw|try|typeof|undefined|var|void|while|with|yield)(?![$\w\xA0-\uFFFF]))(?:(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*\s*)\(\s*|\]\s*\(\s*)(?!\s)(?:[^()\s]|\s+(?![\s)])|\([^()]*\))+(?=\s*\)\s*\{)/,
      lookbehind: !0,
      inside: Prism.languages.javascript
    }
  ],
  constant: /\b[A-Z](?:[A-Z_]|\dx?)*\b/
});
Prism.languages.insertBefore("javascript", "string", {
  hashbang: {
    pattern: /^#!.*/,
    greedy: !0,
    alias: "comment"
  },
  "template-string": {
    pattern: /`(?:\\[\s\S]|\$\{(?:[^{}]|\{(?:[^{}]|\{[^}]*\})*\})+\}|(?!\$\{)[^\\`])*`/,
    greedy: !0,
    inside: {
      "template-punctuation": {
        pattern: /^`|`$/,
        alias: "string"
      },
      interpolation: {
        pattern: /((?:^|[^\\])(?:\\{2})*)\$\{(?:[^{}]|\{(?:[^{}]|\{[^}]*\})*\})+\}/,
        lookbehind: !0,
        inside: {
          "interpolation-punctuation": {
            pattern: /^\$\{|\}$/,
            alias: "punctuation"
          },
          rest: Prism.languages.javascript
        }
      },
      string: /[\s\S]+/
    }
  },
  "string-property": {
    pattern: /((?:^|[,{])[ \t]*)(["'])(?:\\(?:\r\n|[\s\S])|(?!\2)[^\\\r\n])*\2(?=\s*:)/m,
    lookbehind: !0,
    greedy: !0,
    alias: "property"
  }
});
Prism.languages.insertBefore("javascript", "operator", {
  "literal-property": {
    pattern: /((?:^|[,{])[ \t]*)(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?=\s*:)/m,
    lookbehind: !0,
    alias: "property"
  }
});
Prism.languages.markup && (Prism.languages.markup.tag.addInlined("script", "javascript"), Prism.languages.markup.tag.addAttribute(
  /on(?:abort|blur|change|click|composition(?:end|start|update)|dblclick|error|focus(?:in|out)?|key(?:down|up)|load|mouse(?:down|enter|leave|move|out|over|up)|reset|resize|scroll|select|slotchange|submit|unload|wheel)/.source,
  "javascript"
));
Prism.languages.js = Prism.languages.javascript;
(function(r) {
  var e = r.util.clone(r.languages.javascript), t = /(?:\s|\/\/.*(?!.)|\/\*(?:[^*]|\*(?!\/))\*\/)/.source, a = /(?:\{(?:\{(?:\{[^{}]*\}|[^{}])*\}|[^{}])*\})/.source, n = /(?:\{<S>*\.{3}(?:[^{}]|<BRACES>)*\})/.source;
  function s(h, m) {
    return h = h.replace(/<S>/g, function() {
      return t;
    }).replace(/<BRACES>/g, function() {
      return a;
    }).replace(/<SPREAD>/g, function() {
      return n;
    }), RegExp(h, m);
  }
  n = s(n).source, r.languages.jsx = r.languages.extend("markup", e), r.languages.jsx.tag.pattern = s(
    /<\/?(?:[\w.:-]+(?:<S>+(?:[\w.:$-]+(?:=(?:"(?:\\[\s\S]|[^\\"])*"|'(?:\\[\s\S]|[^\\'])*'|[^\s{'"/>=]+|<BRACES>))?|<SPREAD>))*<S>*\/?)?>/.source
  ), r.languages.jsx.tag.inside.tag.pattern = /^<\/?[^\s>\/]*/, r.languages.jsx.tag.inside["attr-value"].pattern = /=(?!\{)(?:"(?:\\[\s\S]|[^\\"])*"|'(?:\\[\s\S]|[^\\'])*'|[^\s'">]+)/, r.languages.jsx.tag.inside.tag.inside["class-name"] = /^[A-Z]\w*(?:\.[A-Z]\w*)*$/, r.languages.jsx.tag.inside.comment = e.comment, r.languages.insertBefore("inside", "attr-name", {
    spread: {
      pattern: s(/<SPREAD>/.source),
      inside: r.languages.jsx
    }
  }, r.languages.jsx.tag), r.languages.insertBefore("inside", "special-attr", {
    script: {
      // Allow for two levels of nesting
      pattern: s(/=<BRACES>/.source),
      alias: "language-javascript",
      inside: {
        "script-punctuation": {
          pattern: /^=(?=\{)/,
          alias: "punctuation"
        },
        rest: r.languages.jsx
      }
    }
  }, r.languages.jsx.tag);
  var u = function(h) {
    return h ? typeof h == "string" ? h : typeof h.content == "string" ? h.content : h.content.map(u).join("") : "";
  }, o = function(h) {
    for (var m = [], v = 0; v < h.length; v++) {
      var b = h[v], k = !1;
      if (typeof b != "string" && (b.type === "tag" && b.content[0] && b.content[0].type === "tag" ? b.content[0].content[0].content === "</" ? m.length > 0 && m[m.length - 1].tagName === u(b.content[0].content[1]) && m.pop() : b.content[b.content.length - 1].content === "/>" || m.push({
        tagName: u(b.content[0].content[1]),
        openedBraces: 0
      }) : m.length > 0 && b.type === "punctuation" && b.content === "{" ? m[m.length - 1].openedBraces++ : m.length > 0 && m[m.length - 1].openedBraces > 0 && b.type === "punctuation" && b.content === "}" ? m[m.length - 1].openedBraces-- : k = !0), (k || typeof b == "string") && m.length > 0 && m[m.length - 1].openedBraces === 0) {
        var S = u(b);
        v < h.length - 1 && (typeof h[v + 1] == "string" || h[v + 1].type === "plain-text") && (S += u(h[v + 1]), h.splice(v + 1, 1)), v > 0 && (typeof h[v - 1] == "string" || h[v - 1].type === "plain-text") && (S = u(h[v - 1]) + S, h.splice(v - 1, 1), v--), h[v] = new r.Token("plain-text", S, null, S);
      }
      b.content && typeof b.content != "string" && o(b.content);
    }
  };
  r.hooks.add("after-tokenize", function(h) {
    h.language !== "jsx" && h.language !== "tsx" || o(h.tokens);
  });
})(Prism);
(function(r) {
  r.languages.typescript = r.languages.extend("javascript", {
    "class-name": {
      pattern: /(\b(?:class|extends|implements|instanceof|interface|new|type)\s+)(?!keyof\b)(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*(?:\s*<(?:[^<>]|<(?:[^<>]|<[^<>]*>)*>)*>)?/,
      lookbehind: !0,
      greedy: !0,
      inside: null
      // see below
    },
    builtin: /\b(?:Array|Function|Promise|any|boolean|console|never|number|string|symbol|unknown)\b/
  }), r.languages.typescript.keyword.push(
    /\b(?:abstract|declare|is|keyof|readonly|require)\b/,
    // keywords that have to be followed by an identifier
    /\b(?:asserts|infer|interface|module|namespace|type)\b(?=\s*(?:[{_$a-zA-Z\xA0-\uFFFF]|$))/,
    // This is for `import type *, {}`
    /\btype\b(?=\s*(?:[\{*]|$))/
  ), delete r.languages.typescript.parameter, delete r.languages.typescript["literal-property"];
  var e = r.languages.extend("typescript", {});
  delete e["class-name"], r.languages.typescript["class-name"].inside = e, r.languages.insertBefore("typescript", "function", {
    decorator: {
      pattern: /@[$\w\xA0-\uFFFF]+/,
      inside: {
        at: {
          pattern: /^@/,
          alias: "operator"
        },
        function: /^[\s\S]+/
      }
    },
    "generic-function": {
      // e.g. foo<T extends "bar" | "baz">( ...
      pattern: /#?(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*\s*<(?:[^<>]|<(?:[^<>]|<[^<>]*>)*>)*>(?=\s*\()/,
      greedy: !0,
      inside: {
        function: /^#?(?!\s)[_$a-zA-Z\xA0-\uFFFF](?:(?!\s)[$\w\xA0-\uFFFF])*/,
        generic: {
          pattern: /<[\s\S]+/,
          // everything after the first <
          alias: "class-name",
          inside: e
        }
      }
    }
  }), r.languages.ts = r.languages.typescript;
})(Prism);
(function(r) {
  var e = r.util.clone(r.languages.typescript);
  r.languages.tsx = r.languages.extend("jsx", e), delete r.languages.tsx.parameter, delete r.languages.tsx["literal-property"];
  var t = r.languages.tsx.tag;
  t.pattern = RegExp(/(^|[^\w$]|(?=<\/))/.source + "(?:" + t.pattern.source + ")", t.pattern.flags), t.lookbehind = !0;
})(Prism);
(function(r) {
  var e = "\\b(?:BASH|BASHOPTS|BASH_ALIASES|BASH_ARGC|BASH_ARGV|BASH_CMDS|BASH_COMPLETION_COMPAT_DIR|BASH_LINENO|BASH_REMATCH|BASH_SOURCE|BASH_VERSINFO|BASH_VERSION|COLORTERM|COLUMNS|COMP_WORDBREAKS|DBUS_SESSION_BUS_ADDRESS|DEFAULTS_PATH|DESKTOP_SESSION|DIRSTACK|DISPLAY|EUID|GDMSESSION|GDM_LANG|GNOME_KEYRING_CONTROL|GNOME_KEYRING_PID|GPG_AGENT_INFO|GROUPS|HISTCONTROL|HISTFILE|HISTFILESIZE|HISTSIZE|HOME|HOSTNAME|HOSTTYPE|IFS|INSTANCE|JOB|LANG|LANGUAGE|LC_ADDRESS|LC_ALL|LC_IDENTIFICATION|LC_MEASUREMENT|LC_MONETARY|LC_NAME|LC_NUMERIC|LC_PAPER|LC_TELEPHONE|LC_TIME|LESSCLOSE|LESSOPEN|LINES|LOGNAME|LS_COLORS|MACHTYPE|MAILCHECK|MANDATORY_PATH|NO_AT_BRIDGE|OLDPWD|OPTERR|OPTIND|ORBIT_SOCKETDIR|OSTYPE|PAPERSIZE|PATH|PIPESTATUS|PPID|PS1|PS2|PS3|PS4|PWD|RANDOM|REPLY|SECONDS|SELINUX_INIT|SESSION|SESSIONTYPE|SESSION_MANAGER|SHELL|SHELLOPTS|SHLVL|SSH_AUTH_SOCK|TERM|UID|UPSTART_EVENTS|UPSTART_INSTANCE|UPSTART_JOB|UPSTART_SESSION|USER|WINDOWID|XAUTHORITY|XDG_CONFIG_DIRS|XDG_CURRENT_DESKTOP|XDG_DATA_DIRS|XDG_GREETER_DATA_DIR|XDG_MENU_PREFIX|XDG_RUNTIME_DIR|XDG_SEAT|XDG_SEAT_PATH|XDG_SESSION_DESKTOP|XDG_SESSION_ID|XDG_SESSION_PATH|XDG_SESSION_TYPE|XDG_VTNR|XMODIFIERS)\\b", t = {
    pattern: /(^(["']?)\w+\2)[ \t]+\S.*/,
    lookbehind: !0,
    alias: "punctuation",
    // this looks reasonably well in all themes
    inside: null
    // see below
  }, a = {
    bash: t,
    environment: {
      pattern: RegExp("\\$" + e),
      alias: "constant"
    },
    variable: [
      // [0]: Arithmetic Environment
      {
        pattern: /\$?\(\([\s\S]+?\)\)/,
        greedy: !0,
        inside: {
          // If there is a $ sign at the beginning highlight $(( and )) as variable
          variable: [
            {
              pattern: /(^\$\(\([\s\S]+)\)\)/,
              lookbehind: !0
            },
            /^\$\(\(/
          ],
          number: /\b0x[\dA-Fa-f]+\b|(?:\b\d+(?:\.\d*)?|\B\.\d+)(?:[Ee]-?\d+)?/,
          // Operators according to https://www.gnu.org/software/bash/manual/bashref.html#Shell-Arithmetic
          operator: /--|\+\+|\*\*=?|<<=?|>>=?|&&|\|\||[=!+\-*/%<>^&|]=?|[?~:]/,
          // If there is no $ sign at the beginning highlight (( and )) as punctuation
          punctuation: /\(\(?|\)\)?|,|;/
        }
      },
      // [1]: Command Substitution
      {
        pattern: /\$\((?:\([^)]+\)|[^()])+\)|`[^`]+`/,
        greedy: !0,
        inside: {
          variable: /^\$\(|^`|\)$|`$/
        }
      },
      // [2]: Brace expansion
      {
        pattern: /\$\{[^}]+\}/,
        greedy: !0,
        inside: {
          operator: /:[-=?+]?|[!\/]|##?|%%?|\^\^?|,,?/,
          punctuation: /[\[\]]/,
          environment: {
            pattern: RegExp("(\\{)" + e),
            lookbehind: !0,
            alias: "constant"
          }
        }
      },
      /\$(?:\w+|[#?*!@$])/
    ],
    // Escape sequences from echo and printf's manuals, and escaped quotes.
    entity: /\\(?:[abceEfnrtv\\"]|O?[0-7]{1,3}|U[0-9a-fA-F]{8}|u[0-9a-fA-F]{4}|x[0-9a-fA-F]{1,2})/
  };
  r.languages.bash = {
    shebang: {
      pattern: /^#!\s*\/.*/,
      alias: "important"
    },
    comment: {
      pattern: /(^|[^"{\\$])#.*/,
      lookbehind: !0
    },
    "function-name": [
      // a) function foo {
      // b) foo() {
      // c) function foo() {
      // but not “foo {”
      {
        // a) and c)
        pattern: /(\bfunction\s+)[\w-]+(?=(?:\s*\(?:\s*\))?\s*\{)/,
        lookbehind: !0,
        alias: "function"
      },
      {
        // b)
        pattern: /\b[\w-]+(?=\s*\(\s*\)\s*\{)/,
        alias: "function"
      }
    ],
    // Highlight variable names as variables in for and select beginnings.
    "for-or-select": {
      pattern: /(\b(?:for|select)\s+)\w+(?=\s+in\s)/,
      alias: "variable",
      lookbehind: !0
    },
    // Highlight variable names as variables in the left-hand part
    // of assignments (“=” and “+=”).
    "assign-left": {
      pattern: /(^|[\s;|&]|[<>]\()\w+(?:\.\w+)*(?=\+?=)/,
      inside: {
        environment: {
          pattern: RegExp("(^|[\\s;|&]|[<>]\\()" + e),
          lookbehind: !0,
          alias: "constant"
        }
      },
      alias: "variable",
      lookbehind: !0
    },
    // Highlight parameter names as variables
    parameter: {
      pattern: /(^|\s)-{1,2}(?:\w+:[+-]?)?\w+(?:\.\w+)*(?=[=\s]|$)/,
      alias: "variable",
      lookbehind: !0
    },
    string: [
      // Support for Here-documents https://en.wikipedia.org/wiki/Here_document
      {
        pattern: /((?:^|[^<])<<-?\s*)(\w+)\s[\s\S]*?(?:\r?\n|\r)\2/,
        lookbehind: !0,
        greedy: !0,
        inside: a
      },
      // Here-document with quotes around the tag
      // → No expansion (so no “inside”).
      {
        pattern: /((?:^|[^<])<<-?\s*)(["'])(\w+)\2\s[\s\S]*?(?:\r?\n|\r)\3/,
        lookbehind: !0,
        greedy: !0,
        inside: {
          bash: t
        }
      },
      // “Normal” string
      {
        // https://www.gnu.org/software/bash/manual/html_node/Double-Quotes.html
        pattern: /(^|[^\\](?:\\\\)*)"(?:\\[\s\S]|\$\([^)]+\)|\$(?!\()|`[^`]+`|[^"\\`$])*"/,
        lookbehind: !0,
        greedy: !0,
        inside: a
      },
      {
        // https://www.gnu.org/software/bash/manual/html_node/Single-Quotes.html
        pattern: /(^|[^$\\])'[^']*'/,
        lookbehind: !0,
        greedy: !0
      },
      {
        // https://www.gnu.org/software/bash/manual/html_node/ANSI_002dC-Quoting.html
        pattern: /\$'(?:[^'\\]|\\[\s\S])*'/,
        greedy: !0,
        inside: {
          entity: a.entity
        }
      }
    ],
    environment: {
      pattern: RegExp("\\$?" + e),
      alias: "constant"
    },
    variable: a.variable,
    function: {
      pattern: /(^|[\s;|&]|[<>]\()(?:add|apropos|apt|apt-cache|apt-get|aptitude|aspell|automysqlbackup|awk|basename|bash|bc|bconsole|bg|bzip2|cal|cargo|cat|cfdisk|chgrp|chkconfig|chmod|chown|chroot|cksum|clear|cmp|column|comm|composer|cp|cron|crontab|csplit|curl|cut|date|dc|dd|ddrescue|debootstrap|df|diff|diff3|dig|dir|dircolors|dirname|dirs|dmesg|docker|docker-compose|du|egrep|eject|env|ethtool|expand|expect|expr|fdformat|fdisk|fg|fgrep|file|find|fmt|fold|format|free|fsck|ftp|fuser|gawk|git|gparted|grep|groupadd|groupdel|groupmod|groups|grub-mkconfig|gzip|halt|head|hg|history|host|hostname|htop|iconv|id|ifconfig|ifdown|ifup|import|install|ip|java|jobs|join|kill|killall|less|link|ln|locate|logname|logrotate|look|lpc|lpr|lprint|lprintd|lprintq|lprm|ls|lsof|lynx|make|man|mc|mdadm|mkconfig|mkdir|mke2fs|mkfifo|mkfs|mkisofs|mknod|mkswap|mmv|more|most|mount|mtools|mtr|mutt|mv|nano|nc|netstat|nice|nl|node|nohup|notify-send|npm|nslookup|op|open|parted|passwd|paste|pathchk|ping|pkill|pnpm|podman|podman-compose|popd|pr|printcap|printenv|ps|pushd|pv|quota|quotacheck|quotactl|ram|rar|rcp|reboot|remsync|rename|renice|rev|rm|rmdir|rpm|rsync|scp|screen|sdiff|sed|sendmail|seq|service|sftp|sh|shellcheck|shuf|shutdown|sleep|slocate|sort|split|ssh|stat|strace|su|sudo|sum|suspend|swapon|sync|sysctl|tac|tail|tar|tee|time|timeout|top|touch|tr|traceroute|tsort|tty|umount|uname|unexpand|uniq|units|unrar|unshar|unzip|update-grub|uptime|useradd|userdel|usermod|users|uudecode|uuencode|v|vcpkg|vdir|vi|vim|virsh|vmstat|wait|watch|wc|wget|whereis|which|who|whoami|write|xargs|xdg-open|yarn|yes|zenity|zip|zsh|zypper)(?=$|[)\s;|&])/,
      lookbehind: !0
    },
    keyword: {
      pattern: /(^|[\s;|&]|[<>]\()(?:case|do|done|elif|else|esac|fi|for|function|if|in|select|then|until|while)(?=$|[)\s;|&])/,
      lookbehind: !0
    },
    // https://www.gnu.org/software/bash/manual/html_node/Shell-Builtin-Commands.html
    builtin: {
      pattern: /(^|[\s;|&]|[<>]\()(?:\.|:|alias|bind|break|builtin|caller|cd|command|continue|declare|echo|enable|eval|exec|exit|export|getopts|hash|help|let|local|logout|mapfile|printf|pwd|read|readarray|readonly|return|set|shift|shopt|source|test|times|trap|type|typeset|ulimit|umask|unalias|unset)(?=$|[)\s;|&])/,
      lookbehind: !0,
      // Alias added to make those easier to distinguish from strings.
      alias: "class-name"
    },
    boolean: {
      pattern: /(^|[\s;|&]|[<>]\()(?:false|true)(?=$|[)\s;|&])/,
      lookbehind: !0
    },
    "file-descriptor": {
      pattern: /\B&\d\b/,
      alias: "important"
    },
    operator: {
      // Lots of redirections here, but not just that.
      pattern: /\d?<>|>\||\+=|=[=~]?|!=?|<<[<-]?|[&\d]?>>|\d[<>]&?|[<>][&=]?|&[>&]?|\|[&|]?/,
      inside: {
        "file-descriptor": {
          pattern: /^\d/,
          alias: "important"
        }
      }
    },
    punctuation: /\$?\(\(?|\)\)?|\.\.|[{}[\];\\]/,
    number: {
      pattern: /(^|\s)(?:[1-9]\d*|0)(?:[.,]\d+)?\b/,
      lookbehind: !0
    }
  }, t.inside = r.languages.bash;
  for (var n = [
    "comment",
    "function-name",
    "for-or-select",
    "assign-left",
    "parameter",
    "string",
    "environment",
    "function",
    "keyword",
    "builtin",
    "boolean",
    "file-descriptor",
    "operator",
    "punctuation",
    "number"
  ], s = a.variable[1].inside, u = 0; u < n.length; u++)
    s[n[u]] = r.languages.bash[n[u]];
  r.languages.sh = r.languages.bash, r.languages.shell = r.languages.bash;
})(Prism);
Prism.languages.json = {
  property: {
    pattern: /(^|[^\\])"(?:\\.|[^\\"\r\n])*"(?=\s*:)/,
    lookbehind: !0,
    greedy: !0
  },
  string: {
    pattern: /(^|[^\\])"(?:\\.|[^\\"\r\n])*"(?!\s*:)/,
    lookbehind: !0,
    greedy: !0
  },
  comment: {
    pattern: /\/\/.*|\/\*[\s\S]*?(?:\*\/|$)/,
    greedy: !0
  },
  number: /-?\b\d+(?:\.\d+)?(?:e[+-]?\d+)?\b/i,
  punctuation: /[{}[\],]/,
  operator: /:/,
  boolean: /\b(?:false|true)\b/,
  null: {
    pattern: /\bnull\b/,
    alias: "keyword"
  }
};
Prism.languages.webmanifest = Prism.languages.json;
const dn = {
  html: "markup",
  js: "javascript",
  jsx: "jsx",
  shell: "bash",
  sh: "bash",
  ts: "typescript",
  tsx: "tsx"
};
function mn(r) {
  const e = (r ?? "tsx").trim().toLowerCase();
  return dn[e] ?? e;
}
function fn(r, e) {
  const t = nt.languages[e];
  return t ? nt.highlight(r || " ", t, e) : nt.util.encode(r || " ").toString();
}
function pn({
  code: r,
  language: e = "tsx",
  showLineNumbers: t = !0
}) {
  const a = mn(e), n = r.replace(/\n$/, "").split(`
`);
  return /* @__PURE__ */ y("code", { className: `ra-code language-${a}`, "data-language": a, children: n.map((s, u) => /* @__PURE__ */ $("span", { className: "ra-code__line", children: [
    t ? /* @__PURE__ */ y("span", { className: "ra-code__line-no", "aria-hidden": "true", children: u + 1 }) : null,
    /* @__PURE__ */ y(
      "span",
      {
        className: "ra-code__line-code",
        dangerouslySetInnerHTML: { __html: fn(s, a) }
      }
    )
  ] }, u)) });
}
function Zs({
  code: r,
  language: e = "tsx",
  title: t,
  showLineNumbers: a = !0,
  copyable: n = !0
}) {
  const [s, u] = ze(!1), o = r != null && r !== "", h = e.trim().toLowerCase();
  return /* @__PURE__ */ $("div", { className: "ra-codeblock", children: [
    /* @__PURE__ */ $("div", { className: "ra-codeblock__bar", children: [
      /* @__PURE__ */ y("span", { className: "ra-codeblock__lang", children: t ?? h }),
      n ? /* @__PURE__ */ y("button", { className: "ra-codeblock__copy", type: "button", onClick: async () => {
        if (!o) return;
        const v = await Yr(r);
        u(v), window.setTimeout(() => u(!1), 1400);
      }, children: s ? "已复制" : "复制" }) : null
    ] }),
    o ? /* @__PURE__ */ y("pre", { className: "ra-codeblock__code", children: /* @__PURE__ */ y(
      pn,
      {
        code: r,
        language: h,
        showLineNumbers: a
      }
    ) }) : /* @__PURE__ */ y("div", { className: "ra-codeblock__missing", children: /* @__PURE__ */ y(J, { label: "代码内容" }) })
  ] });
}
class E extends Error {
  // The underlying error message without any context added.
  constructor(e, t) {
    var a = "KaTeX parse error: " + e, n, s, u = t && t.loc;
    if (u && u.start <= u.end) {
      var o = u.lexer.input;
      n = u.start, s = u.end, n === o.length ? a += " at end of input: " : a += " at position " + (n + 1) + ": ";
      var h = o.slice(n, s).replace(/[^]/g, "$&̲"), m;
      n > 15 ? m = "…" + o.slice(n - 15, n) : m = o.slice(0, n);
      var v;
      s + 15 < o.length ? v = o.slice(s, s + 15) + "…" : v = o.slice(s), a += m + h + v;
    }
    super(a), this.name = "ParseError", this.position = void 0, this.length = void 0, this.rawMessage = void 0, Object.setPrototypeOf(this, E.prototype), this.position = n, n != null && s != null && (this.length = s - n), this.rawMessage = e;
  }
}
var vn = /([A-Z])/g, gn = (r) => r.replace(vn, "-$1").toLowerCase(), bn = {
  "&": "&amp;",
  ">": "&gt;",
  "<": "&lt;",
  '"': "&quot;",
  "'": "&#x27;"
}, yn = /[&><"']/g, d0 = (r) => String(r).replace(yn, (e) => bn[e]), $e = (r) => r.type === "ordgroup" || r.type === "color" ? r.body.length === 1 ? $e(r.body[0]) : r : r.type === "font" ? $e(r.body) : r, xn = /* @__PURE__ */ new Set(["mathord", "textord", "atom"]), U0 = (r) => xn.has($e(r).type), wn = (r) => {
  var e = /^[\x00-\x20]*([^\\/#?]*?)(:|&#0*58|&#x0*3a|&colon)/i.exec(r);
  return e ? e[2] !== ":" || !/^[a-zA-Z][a-zA-Z0-9+\-.]*$/.test(e[1]) ? null : e[1].toLowerCase() : "_relative";
}, kt = {
  displayMode: {
    type: "boolean",
    description: "Render math in display mode, which puts the math in display style (so \\int and \\sum are large, for example), and centers the math on the page on its own line.",
    cli: "-d, --display-mode"
  },
  output: {
    type: {
      enum: ["htmlAndMathml", "html", "mathml"]
    },
    description: "Determines the markup language of the output.",
    cli: "-F, --format <type>"
  },
  leqno: {
    type: "boolean",
    description: "Render display math in leqno style (left-justified tags)."
  },
  fleqn: {
    type: "boolean",
    description: "Render display math flush left."
  },
  throwOnError: {
    type: "boolean",
    default: !0,
    cli: "-t, --no-throw-on-error",
    cliDescription: "Render errors (in the color given by --error-color) instead of throwing a ParseError exception when encountering an error."
  },
  errorColor: {
    type: "string",
    default: "#cc0000",
    cli: "-c, --error-color <color>",
    cliDescription: "A color string given in the format 'rgb' or 'rrggbb' (no #). This option determines the color of errors rendered by the -t option.",
    cliProcessor: (r) => "#" + r
  },
  macros: {
    type: "object",
    cli: "-m, --macro <def>",
    cliDescription: "Define custom macro of the form '\\foo:expansion' (use multiple -m arguments for multiple macros).",
    cliDefault: [],
    cliProcessor: (r, e) => (e.push(r), e)
  },
  minRuleThickness: {
    type: "number",
    description: "Specifies a minimum thickness, in ems, for fraction lines, `\\sqrt` top lines, `{array}` vertical lines, `\\hline`, `\\hdashline`, `\\underline`, `\\overline`, and the borders of `\\fbox`, `\\boxed`, and `\\fcolorbox`.",
    processor: (r) => Math.max(0, r),
    cli: "--min-rule-thickness <size>",
    cliProcessor: parseFloat
  },
  colorIsTextColor: {
    type: "boolean",
    description: "Makes \\color behave like LaTeX's 2-argument \\textcolor, instead of LaTeX's one-argument \\color mode change.",
    cli: "-b, --color-is-text-color"
  },
  strict: {
    type: [{
      enum: ["warn", "ignore", "error"]
    }, "boolean", "function"],
    description: "Turn on strict / LaTeX faithfulness mode, which throws an error if the input uses features that are not supported by LaTeX.",
    cli: "-S, --strict",
    cliDefault: !1
  },
  trust: {
    type: ["boolean", "function"],
    description: "Trust the input, enabling all HTML features such as \\url.",
    cli: "-T, --trust"
  },
  maxSize: {
    type: "number",
    default: 1 / 0,
    description: "If non-zero, all user-specified sizes, e.g. in \\rule{500em}{500em}, will be capped to maxSize ems. Otherwise, elements and spaces can be arbitrarily large",
    processor: (r) => Math.max(0, r),
    cli: "-s, --max-size <n>",
    cliProcessor: parseInt
  },
  maxExpand: {
    type: "number",
    default: 1e3,
    description: "Limit the number of macro expansions to the specified number, to prevent e.g. infinite macro loops. If set to Infinity, the macro expander will try to fully expand as in LaTeX.",
    processor: (r) => Math.max(0, r),
    cli: "-e, --max-expand <n>",
    cliProcessor: (r) => r === "Infinity" ? 1 / 0 : parseInt(r)
  },
  globalGroup: {
    type: "boolean",
    cli: !1
  }
};
function Sn(r) {
  if (typeof r != "string")
    return r.enum[0];
  switch (r) {
    case "boolean":
      return !1;
    case "string":
      return "";
    case "number":
      return 0;
    case "object":
      return {};
    default:
      throw new Error("Unexpected schema type; settings must declare an explicit default.");
  }
}
function kn(r) {
  if (r.default !== void 0)
    return r.default;
  var e = Array.isArray(r.type) ? r.type[0] : r.type;
  return Sn(e);
}
function An(r, e, t, a) {
  var n = t[e];
  r[e] = n !== void 0 ? a.processor ? a.processor(n) : n : kn(a);
}
class Gt {
  constructor(e) {
    e === void 0 && (e = {}), this.displayMode = void 0, this.output = void 0, this.leqno = void 0, this.fleqn = void 0, this.throwOnError = void 0, this.errorColor = void 0, this.macros = void 0, this.minRuleThickness = void 0, this.colorIsTextColor = void 0, this.strict = void 0, this.trust = void 0, this.maxSize = void 0, this.maxExpand = void 0, this.globalGroup = void 0, e = e || {};
    for (var t of Object.keys(kt)) {
      var a = kt[t];
      a && An(this, t, e, a);
    }
  }
  /**
   * Report nonstrict (non-LaTeX-compatible) input.
   * Can safely not be called if `this.strict` is false in JavaScript.
   */
  reportNonstrict(e, t, a) {
    var n = this.strict;
    if (typeof n == "function" && (n = n(e, t, a)), !(!n || n === "ignore")) {
      if (n === !0 || n === "error")
        throw new E("LaTeX-incompatible input and strict mode is set to 'error': " + (t + " [" + e + "]"), a);
      n === "warn" ? typeof console < "u" && console.warn("LaTeX-incompatible input and strict mode is set to 'warn': " + (t + " [" + e + "]")) : typeof console < "u" && console.warn("LaTeX-incompatible input and strict mode is set to " + ("unrecognized '" + n + "': " + t + " [" + e + "]"));
    }
  }
  /**
   * Check whether to apply strict (LaTeX-adhering) behavior for unusual
   * input (like `\\`).  Unlike `nonstrict`, will not throw an error;
   * instead, "error" translates to a return value of `true`, while "ignore"
   * translates to a return value of `false`.  May still print a warning:
   * "warn" prints a warning and returns `false`.
   * This is for the second category of `errorCode`s listed in the README.
   */
  useStrictBehavior(e, t, a) {
    var n = this.strict;
    if (typeof n == "function")
      try {
        n = n(e, t, a);
      } catch {
        n = "error";
      }
    return !n || n === "ignore" ? !1 : n === !0 || n === "error" ? !0 : n === "warn" ? (typeof console < "u" && console.warn("LaTeX-incompatible input and strict mode is set to 'warn': " + (t + " [" + e + "]")), !1) : (typeof console < "u" && console.warn("LaTeX-incompatible input and strict mode is set to " + ("unrecognized '" + n + "': " + t + " [" + e + "]")), !1);
  }
  /**
   * Check whether to test potentially dangerous input, and return
   * `true` (trusted) or `false` (untrusted).  The sole argument `context`
   * should be an object with `command` field specifying the relevant LaTeX
   * command (as a string starting with `\`), and any other arguments, etc.
   * If `context` has a `url` field, a `protocol` field will automatically
   * get added by this function (changing the specified object).
   */
  isTrusted(e) {
    if ("url" in e && e.url && !e.protocol) {
      var t = wn(e.url);
      if (t == null)
        return !1;
      e.protocol = t;
    }
    var a = typeof this.trust == "function" ? this.trust(e) : this.trust;
    return !!a;
  }
}
class Z0 {
  constructor(e, t, a) {
    this.id = void 0, this.size = void 0, this.cramped = void 0, this.id = e, this.size = t, this.cramped = a;
  }
  /**
   * Get the style of a superscript given a base in the current style.
   */
  sup() {
    return B0[Tn[this.id]];
  }
  /**
   * Get the style of a subscript given a base in the current style.
   */
  sub() {
    return B0[zn[this.id]];
  }
  /**
   * Get the style of a fraction numerator given the fraction in the current
   * style.
   */
  fracNum() {
    return B0[Mn[this.id]];
  }
  /**
   * Get the style of a fraction denominator given the fraction in the current
   * style.
   */
  fracDen() {
    return B0[Fn[this.id]];
  }
  /**
   * Get the cramped version of a style (in particular, cramping a cramped style
   * doesn't change the style).
   */
  cramp() {
    return B0[Nn[this.id]];
  }
  /**
   * Get a text or display version of this style.
   */
  text() {
    return B0[Cn[this.id]];
  }
  /**
   * Return true if this style is tightly spaced (scriptstyle/scriptscriptstyle)
   */
  isTight() {
    return this.size >= 2;
  }
}
var Ut = 0, Ge = 1, me = 2, P0 = 3, Se = 4, N0 = 5, fe = 6, v0 = 7, B0 = [new Z0(Ut, 0, !1), new Z0(Ge, 0, !0), new Z0(me, 1, !1), new Z0(P0, 1, !0), new Z0(Se, 2, !1), new Z0(N0, 2, !0), new Z0(fe, 3, !1), new Z0(v0, 3, !0)], Tn = [Se, N0, Se, N0, fe, v0, fe, v0], zn = [N0, N0, N0, N0, v0, v0, v0, v0], Mn = [me, P0, Se, N0, fe, v0, fe, v0], Fn = [P0, P0, N0, N0, v0, v0, v0, v0], Nn = [Ge, Ge, P0, P0, N0, N0, v0, v0], Cn = [Ut, Ge, me, P0, me, P0, me, P0], U = {
  DISPLAY: B0[Ut],
  TEXT: B0[me],
  SCRIPT: B0[Se],
  SCRIPTSCRIPT: B0[fe]
}, At = [{
  // Latin characters beyond the Latin-1 characters we have metrics for.
  // Needed for Czech, Hungarian and Turkish text, for example.
  name: "latin",
  blocks: [
    [256, 591],
    // Latin Extended-A and Latin Extended-B
    [768, 879]
    // Combining Diacritical marks
  ]
}, {
  // The Cyrillic script used by Russian and related languages.
  // A Cyrillic subset used to be supported as explicitly defined
  // symbols in symbols.js
  name: "cyrillic",
  blocks: [[1024, 1279]]
}, {
  // Armenian
  name: "armenian",
  blocks: [[1328, 1423]]
}, {
  // The Brahmic scripts of South and Southeast Asia
  // Devanagari (0900–097F)
  // Bengali (0980–09FF)
  // Gurmukhi (0A00–0A7F)
  // Gujarati (0A80–0AFF)
  // Oriya (0B00–0B7F)
  // Tamil (0B80–0BFF)
  // Telugu (0C00–0C7F)
  // Kannada (0C80–0CFF)
  // Malayalam (0D00–0D7F)
  // Sinhala (0D80–0DFF)
  // Thai (0E00–0E7F)
  // Lao (0E80–0EFF)
  // Tibetan (0F00–0FFF)
  // Myanmar (1000–109F)
  name: "brahmic",
  blocks: [[2304, 4255]]
}, {
  name: "georgian",
  blocks: [[4256, 4351]]
}, {
  // Chinese and Japanese.
  // The "k" in cjk is for Korean, but we've separated Korean out
  name: "cjk",
  blocks: [
    [12288, 12543],
    // CJK symbols and punctuation, Hiragana, Katakana
    [19968, 40879],
    // CJK ideograms
    [65280, 65376]
    // Fullwidth punctuation
    // TODO: add halfwidth Katakana and Romanji glyphs
  ]
}, {
  // Korean
  name: "hangul",
  blocks: [[44032, 55215]]
}];
function En(r) {
  for (var e = 0; e < At.length; e++)
    for (var t = At[e], a = 0; a < t.blocks.length; a++) {
      var n = t.blocks[a];
      if (r >= n[0] && r <= n[1])
        return t.name;
    }
  return null;
}
var Pe = [];
At.forEach((r) => r.blocks.forEach((e) => Pe.push(...e)));
function Zr(r) {
  for (var e = 0; e < Pe.length; e += 2)
    if (r >= Pe[e] && r <= Pe[e + 1])
      return !0;
  return !1;
}
var u0 = (r) => r + " " + r, de = 80, Dn = function(e, t) {
  return "M95," + (622 + e + t) + `
c-2.7,0,-7.17,-2.7,-13.5,-8c-5.8,-5.3,-9.5,-10,-9.5,-14
c0,-2,0.3,-3.3,1,-4c1.3,-2.7,23.83,-20.7,67.5,-54
c44.2,-33.3,65.8,-50.3,66.5,-51c1.3,-1.3,3,-2,5,-2c4.7,0,8.7,3.3,12,10
s173,378,173,378c0.7,0,35.3,-71,104,-213c68.7,-142,137.5,-285,206.5,-429
c69,-144,104.5,-217.7,106.5,-221
l` + e / 2.075 + " -" + e + `
c5.3,-9.3,12,-14,20,-14
H400000v` + (40 + e) + `H845.2724
s-225.272,467,-225.272,467s-235,486,-235,486c-2.7,4.7,-9,7,-19,7
c-6,0,-10,-1,-12,-3s-194,-422,-194,-422s-65,47,-65,47z
M` + (834 + e) + " " + t + "h400000v" + (40 + e) + "h-400000z";
}, Bn = function(e, t) {
  return "M263," + (601 + e + t) + `c0.7,0,18,39.7,52,119
c34,79.3,68.167,158.7,102.5,238c34.3,79.3,51.8,119.3,52.5,120
c340,-704.7,510.7,-1060.3,512,-1067
l` + e / 2.084 + " -" + e + `
c4.7,-7.3,11,-11,19,-11
H40000v` + (40 + e) + `H1012.3
s-271.3,567,-271.3,567c-38.7,80.7,-84,175,-136,283c-52,108,-89.167,185.3,-111.5,232
c-22.3,46.7,-33.8,70.3,-34.5,71c-4.7,4.7,-12.3,7,-23,7s-12,-1,-12,-1
s-109,-253,-109,-253c-72.7,-168,-109.3,-252,-110,-252c-10.7,8,-22,16.7,-34,26
c-22,17.3,-33.3,26,-34,26s-26,-26,-26,-26s76,-59,76,-59s76,-60,76,-60z
M` + (1001 + e) + " " + t + "h400000v" + (40 + e) + "h-400000z";
}, In = function(e, t) {
  return "M983 " + (10 + e + t) + `
l` + e / 3.13 + " -" + e + `
c4,-6.7,10,-10,18,-10 H400000v` + (40 + e) + `
H1013.1s-83.4,268,-264.1,840c-180.7,572,-277,876.3,-289,913c-4.7,4.7,-12.7,7,-24,7
s-12,0,-12,0c-1.3,-3.3,-3.7,-11.7,-7,-25c-35.3,-125.3,-106.7,-373.3,-214,-744
c-10,12,-21,25,-33,39s-32,39,-32,39c-6,-5.3,-15,-14,-27,-26s25,-30,25,-30
c26.7,-32.7,52,-63,76,-91s52,-60,52,-60s208,722,208,722
c56,-175.3,126.3,-397.3,211,-666c84.7,-268.7,153.8,-488.2,207.5,-658.5
c53.7,-170.3,84.5,-266.8,92.5,-289.5z
M` + (1001 + e) + " " + t + "h400000v" + (40 + e) + "h-400000z";
}, Rn = function(e, t) {
  return "M424," + (2398 + e + t) + `
c-1.3,-0.7,-38.5,-172,-111.5,-514c-73,-342,-109.8,-513.3,-110.5,-514
c0,-2,-10.7,14.3,-32,49c-4.7,7.3,-9.8,15.7,-15.5,25c-5.7,9.3,-9.8,16,-12.5,20
s-5,7,-5,7c-4,-3.3,-8.3,-7.7,-13,-13s-13,-13,-13,-13s76,-122,76,-122s77,-121,77,-121
s209,968,209,968c0,-2,84.7,-361.7,254,-1079c169.3,-717.3,254.7,-1077.7,256,-1081
l` + e / 4.223 + " -" + e + `c4,-6.7,10,-10,18,-10 H400000
v` + (40 + e) + `H1014.6
s-87.3,378.7,-272.6,1166c-185.3,787.3,-279.3,1182.3,-282,1185
c-2,6,-10,9,-24,9
c-8,0,-12,-0.7,-12,-2z M` + (1001 + e) + " " + t + `
h400000v` + (40 + e) + "h-400000z";
}, qn = function(e, t) {
  return "M473," + (2713 + e + t) + `
c339.3,-1799.3,509.3,-2700,510,-2702 l` + e / 5.298 + " -" + e + `
c3.3,-7.3,9.3,-11,18,-11 H400000v` + (40 + e) + `H1017.7
s-90.5,478,-276.2,1466c-185.7,988,-279.5,1483,-281.5,1485c-2,6,-10,9,-24,9
c-8,0,-12,-0.7,-12,-2c0,-1.3,-5.3,-32,-16,-92c-50.7,-293.3,-119.7,-693.3,-207,-1200
c0,-1.3,-5.3,8.7,-16,30c-10.7,21.3,-21.3,42.7,-32,64s-16,33,-16,33s-26,-26,-26,-26
s76,-153,76,-153s77,-151,77,-151c0.7,0.7,35.7,202,105,604c67.3,400.7,102,602.7,104,
606zM` + (1001 + e) + " " + t + "h400000v" + (40 + e) + "H1017.7z";
}, _n = function(e) {
  var t = e / 2;
  return "M400000 " + e + " H0 L" + t + " 0 l65 45 L145 " + (e - 80) + " H400000z";
}, On = function(e, t, a) {
  var n = a - 54 - t - e;
  return "M702 " + (e + t) + "H400000" + (40 + e) + `
H742v` + n + `l-4 4-4 4c-.667.7 -2 1.5-4 2.5s-4.167 1.833-6.5 2.5-5.5 1-9.5 1
h-12l-28-84c-16.667-52-96.667 -294.333-240-727l-212 -643 -85 170
c-4-3.333-8.333-7.667-13 -13l-13-13l77-155 77-156c66 199.333 139 419.667
219 661 l218 661zM702 ` + t + "H400000v" + (40 + e) + "H742z";
}, Ln = function(e, t, a) {
  t = 1e3 * t;
  var n = "";
  switch (e) {
    case "sqrtMain":
      n = Dn(t, de);
      break;
    case "sqrtSize1":
      n = Bn(t, de);
      break;
    case "sqrtSize2":
      n = In(t, de);
      break;
    case "sqrtSize3":
      n = Rn(t, de);
      break;
    case "sqrtSize4":
      n = qn(t, de);
      break;
    case "sqrtTall":
      n = On(t, de, a);
  }
  return n;
}, Hn = function(e, t) {
  switch (e) {
    case "⎜":
      return u0("M291 0 H417 V" + t + " H291z");
    case "∣":
      return u0("M145 0 H188 V" + t + " H145z");
    case "∥":
      return u0("M145 0 H188 V" + t + " H145z") + u0("M367 0 H410 V" + t + " H367z");
    case "⎟":
      return u0("M457 0 H583 V" + t + " H457z");
    case "⎢":
      return u0("M319 0 H403 V" + t + " H319z");
    case "⎥":
      return u0("M263 0 H347 V" + t + " H263z");
    case "⎪":
      return u0("M384 0 H504 V" + t + " H384z");
    case "⏐":
      return u0("M312 0 H355 V" + t + " H312z");
    case "‖":
      return u0("M257 0 H300 V" + t + " H257z") + u0("M478 0 H521 V" + t + " H478z");
    default:
      return "";
  }
}, or = {
  // The doubleleftarrow geometry is from glyph U+21D0 in the font KaTeX Main
  doubleleftarrow: `M262 157
l10-10c34-36 62.7-77 86-123 3.3-8 5-13.3 5-16 0-5.3-6.7-8-20-8-7.3
 0-12.2.5-14.5 1.5-2.3 1-4.8 4.5-7.5 10.5-49.3 97.3-121.7 169.3-217 216-28
 14-57.3 25-88 33-6.7 2-11 3.8-13 5.5-2 1.7-3 4.2-3 7.5s1 5.8 3 7.5
c2 1.7 6.3 3.5 13 5.5 68 17.3 128.2 47.8 180.5 91.5 52.3 43.7 93.8 96.2 124.5
 157.5 9.3 8 15.3 12.3 18 13h6c12-.7 18-4 18-10 0-2-1.7-7-5-15-23.3-46-52-87
-86-123l-10-10h399738v-40H218c328 0 0 0 0 0l-10-8c-26.7-20-65.7-43-117-69 2.7
-2 6-3.7 10-5 36.7-16 72.3-37.3 107-64l10-8h399782v-40z
m8 0v40h399730v-40zm0 194v40h399730v-40z`,
  // doublerightarrow is from glyph U+21D2 in font KaTeX Main
  doublerightarrow: `M399738 392l
-10 10c-34 36-62.7 77-86 123-3.3 8-5 13.3-5 16 0 5.3 6.7 8 20 8 7.3 0 12.2-.5
 14.5-1.5 2.3-1 4.8-4.5 7.5-10.5 49.3-97.3 121.7-169.3 217-216 28-14 57.3-25 88
-33 6.7-2 11-3.8 13-5.5 2-1.7 3-4.2 3-7.5s-1-5.8-3-7.5c-2-1.7-6.3-3.5-13-5.5-68
-17.3-128.2-47.8-180.5-91.5-52.3-43.7-93.8-96.2-124.5-157.5-9.3-8-15.3-12.3-18
-13h-6c-12 .7-18 4-18 10 0 2 1.7 7 5 15 23.3 46 52 87 86 123l10 10H0v40h399782
c-328 0 0 0 0 0l10 8c26.7 20 65.7 43 117 69-2.7 2-6 3.7-10 5-36.7 16-72.3 37.3
-107 64l-10 8H0v40zM0 157v40h399730v-40zm0 194v40h399730v-40z`,
  // leftarrow is from glyph U+2190 in font KaTeX Main
  leftarrow: `M400000 241H110l3-3c68.7-52.7 113.7-120
 135-202 4-14.7 6-23 6-25 0-7.3-7-11-21-11-8 0-13.2.8-15.5 2.5-2.3 1.7-4.2 5.8
-5.5 12.5-1.3 4.7-2.7 10.3-4 17-12 48.7-34.8 92-68.5 130S65.3 228.3 18 247
c-10 4-16 7.7-18 11 0 8.7 6 14.3 18 17 47.3 18.7 87.8 47 121.5 85S196 441.3 208
 490c.7 2 1.3 5 2 9s1.2 6.7 1.5 8c.3 1.3 1 3.3 2 6s2.2 4.5 3.5 5.5c1.3 1 3.3
 1.8 6 2.5s6 1 10 1c14 0 21-3.7 21-11 0-2-2-10.3-6-25-20-79.3-65-146.7-135-202
 l-3-3h399890zM100 241v40h399900v-40z`,
  // overbrace is from glyphs U+23A9/23A8/23A7 in font KaTeX_Size4-Regular
  leftbrace: `M6 548l-6-6v-35l6-11c56-104 135.3-181.3 238-232 57.3-28.7 117
-45 179-50h399577v120H403c-43.3 7-81 15-113 26-100.7 33-179.7 91-237 174-2.7
 5-6 9-10 13-.7 1-7.3 1-20 1H6z`,
  leftbraceunder: `M0 6l6-6h17c12.688 0 19.313.3 20 1 4 4 7.313 8.3 10 13
 35.313 51.3 80.813 93.8 136.5 127.5 55.688 33.7 117.188 55.8 184.5 66.5.688
 0 2 .3 4 1 18.688 2.7 76 4.3 172 5h399450v120H429l-6-1c-124.688-8-235-61.7
-331-161C60.687 138.7 32.312 99.3 7 54L0 41V6z`,
  // overgroup is from the MnSymbol package (public domain)
  leftgroup: `M400000 80
H435C64 80 168.3 229.4 21 260c-5.9 1.2-18 0-18 0-2 0-3-1-3-3v-38C76 61 257 0
 435 0h399565z`,
  leftgroupunder: `M400000 262
H435C64 262 168.3 112.6 21 82c-5.9-1.2-18 0-18 0-2 0-3 1-3 3v38c76 158 257 219
 435 219h399565z`,
  // Harpoons are from glyph U+21BD in font KaTeX Main
  leftharpoon: `M0 267c.7 5.3 3 10 7 14h399993v-40H93c3.3
-3.3 10.2-9.5 20.5-18.5s17.8-15.8 22.5-20.5c50.7-52 88-110.3 112-175 4-11.3 5
-18.3 3-21-1.3-4-7.3-6-18-6-8 0-13 .7-15 2s-4.7 6.7-8 16c-42 98.7-107.3 174.7
-196 228-6.7 4.7-10.7 8-12 10-1.3 2-2 5.7-2 11zm100-26v40h399900v-40z`,
  leftharpoonplus: `M0 267c.7 5.3 3 10 7 14h399993v-40H93c3.3-3.3 10.2-9.5
 20.5-18.5s17.8-15.8 22.5-20.5c50.7-52 88-110.3 112-175 4-11.3 5-18.3 3-21-1.3
-4-7.3-6-18-6-8 0-13 .7-15 2s-4.7 6.7-8 16c-42 98.7-107.3 174.7-196 228-6.7 4.7
-10.7 8-12 10-1.3 2-2 5.7-2 11zm100-26v40h399900v-40zM0 435v40h400000v-40z
m0 0v40h400000v-40z`,
  leftharpoondown: `M7 241c-4 4-6.333 8.667-7 14 0 5.333.667 9 2 11s5.333
 5.333 12 10c90.667 54 156 130 196 228 3.333 10.667 6.333 16.333 9 17 2 .667 5
 1 9 1h5c10.667 0 16.667-2 18-6 2-2.667 1-9.667-3-21-32-87.333-82.667-157.667
-152-211l-3-3h399907v-40zM93 281 H400000 v-40L7 241z`,
  leftharpoondownplus: `M7 435c-4 4-6.3 8.7-7 14 0 5.3.7 9 2 11s5.3 5.3 12
 10c90.7 54 156 130 196 228 3.3 10.7 6.3 16.3 9 17 2 .7 5 1 9 1h5c10.7 0 16.7
-2 18-6 2-2.7 1-9.7-3-21-32-87.3-82.7-157.7-152-211l-3-3h399907v-40H7zm93 0
v40h399900v-40zM0 241v40h399900v-40zm0 0v40h399900v-40z`,
  // hook is from glyph U+21A9 in font KaTeX Main
  lefthook: `M400000 281 H103s-33-11.2-61-33.5S0 197.3 0 164s14.2-61.2 42.5
-83.5C70.8 58.2 104 47 142 47 c16.7 0 25 6.7 25 20 0 12-8.7 18.7-26 20-40 3.3
-68.7 15.7-86 37-10 12-15 25.3-15 40 0 22.7 9.8 40.7 29.5 54 19.7 13.3 43.5 21
 71.5 23h399859zM103 281v-40h399897v40z`,
  leftlinesegment: u0("M40 281 V428 H0 V94 H40 V241 H400000 v40z"),
  leftbracketunder: u0("M0 0 h120 V290 H399995 v120 H0z"),
  leftbracketover: u0("M0 440 h120 V150 H399995 v-120 H0z"),
  leftmapsto: u0("M40 281 V448H0V74H40V241H400000v40z"),
  // tofrom is from glyph U+21C4 in font KaTeX AMS Regular
  leftToFrom: `M0 147h400000v40H0zm0 214c68 40 115.7 95.7 143 167h22c15.3 0 23
-.3 23-1 0-1.3-5.3-13.7-16-37-18-35.3-41.3-69-70-101l-7-8h399905v-40H95l7-8
c28.7-32 52-65.7 70-101 10.7-23.3 16-35.7 16-37 0-.7-7.7-1-23-1h-22C115.7 265.3
 68 321 0 361zm0-174v-40h399900v40zm100 154v40h399900v-40z`,
  longequal: u0("M0 50 h400000 v40H0z m0 194h40000v40H0z"),
  midbrace: `M200428 334
c-100.7-8.3-195.3-44-280-108-55.3-42-101.7-93-139-153l-9-14c-2.7 4-5.7 8.7-9 14
-53.3 86.7-123.7 153-211 199-66.7 36-137.3 56.3-212 62H0V214h199568c178.3-11.7
 311.7-78.3 403-201 6-8 9.7-12 11-12 .7-.7 6.7-1 18-1s17.3.3 18 1c1.3 0 5 4 11
 12 44.7 59.3 101.3 106.3 170 141s145.3 54.3 229 60h199572v120z`,
  midbraceunder: `M199572 214
c100.7 8.3 195.3 44 280 108 55.3 42 101.7 93 139 153l9 14c2.7-4 5.7-8.7 9-14
 53.3-86.7 123.7-153 211-199 66.7-36 137.3-56.3 212-62h199568v120H200432c-178.3
 11.7-311.7 78.3-403 201-6 8-9.7 12-11 12-.7.7-6.7 1-18 1s-17.3-.3-18-1c-1.3 0
-5-4-11-12-44.7-59.3-101.3-106.3-170-141s-145.3-54.3-229-60H0V214z`,
  oiintSize1: `M512.6 71.6c272.6 0 320.3 106.8 320.3 178.2 0 70.8-47.7 177.6
-320.3 177.6S193.1 320.6 193.1 249.8c0-71.4 46.9-178.2 319.5-178.2z
m368.1 178.2c0-86.4-60.9-215.4-368.1-215.4-306.4 0-367.3 129-367.3 215.4 0 85.8
60.9 214.8 367.3 214.8 307.2 0 368.1-129 368.1-214.8z`,
  oiintSize2: `M757.8 100.1c384.7 0 451.1 137.6 451.1 230 0 91.3-66.4 228.8
-451.1 228.8-386.3 0-452.7-137.5-452.7-228.8 0-92.4 66.4-230 452.7-230z
m502.4 230c0-111.2-82.4-277.2-502.4-277.2s-504 166-504 277.2
c0 110 84 276 504 276s502.4-166 502.4-276z`,
  oiiintSize1: `M681.4 71.6c408.9 0 480.5 106.8 480.5 178.2 0 70.8-71.6 177.6
-480.5 177.6S202.1 320.6 202.1 249.8c0-71.4 70.5-178.2 479.3-178.2z
m525.8 178.2c0-86.4-86.8-215.4-525.7-215.4-437.9 0-524.7 129-524.7 215.4 0
85.8 86.8 214.8 524.7 214.8 438.9 0 525.7-129 525.7-214.8z`,
  oiiintSize2: `M1021.2 53c603.6 0 707.8 165.8 707.8 277.2 0 110-104.2 275.8
-707.8 275.8-606 0-710.2-165.8-710.2-275.8C311 218.8 415.2 53 1021.2 53z
m770.4 277.1c0-131.2-126.4-327.6-770.5-327.6S248.4 198.9 248.4 330.1
c0 130 128.8 326.4 772.7 326.4s770.5-196.4 770.5-326.4z`,
  rightarrow: `M0 241v40h399891c-47.3 35.3-84 78-110 128
-16.7 32-27.7 63.7-33 95 0 1.3-.2 2.7-.5 4-.3 1.3-.5 2.3-.5 3 0 7.3 6.7 11 20
 11 8 0 13.2-.8 15.5-2.5 2.3-1.7 4.2-5.5 5.5-11.5 2-13.3 5.7-27 11-41 14.7-44.7
 39-84.5 73-119.5s73.7-60.2 119-75.5c6-2 9-5.7 9-11s-3-9-9-11c-45.3-15.3-85
-40.5-119-75.5s-58.3-74.8-73-119.5c-4.7-14-8.3-27.3-11-40-1.3-6.7-3.2-10.8-5.5
-12.5-2.3-1.7-7.5-2.5-15.5-2.5-14 0-21 3.7-21 11 0 2 2 10.3 6 25 20.7 83.3 67
 151.7 139 205zm0 0v40h399900v-40z`,
  rightbrace: `M400000 542l
-6 6h-17c-12.7 0-19.3-.3-20-1-4-4-7.3-8.3-10-13-35.3-51.3-80.8-93.8-136.5-127.5
s-117.2-55.8-184.5-66.5c-.7 0-2-.3-4-1-18.7-2.7-76-4.3-172-5H0V214h399571l6 1
c124.7 8 235 61.7 331 161 31.3 33.3 59.7 72.7 85 118l7 13v35z`,
  rightbraceunder: `M399994 0l6 6v35l-6 11c-56 104-135.3 181.3-238 232-57.3
 28.7-117 45-179 50H-300V214h399897c43.3-7 81-15 113-26 100.7-33 179.7-91 237
-174 2.7-5 6-9 10-13 .7-1 7.3-1 20-1h17z`,
  rightgroup: `M0 80h399565c371 0 266.7 149.4 414 180 5.9 1.2 18 0 18 0 2 0
 3-1 3-3v-38c-76-158-257-219-435-219H0z`,
  rightgroupunder: `M0 262h399565c371 0 266.7-149.4 414-180 5.9-1.2 18 0 18
 0 2 0 3 1 3 3v38c-76 158-257 219-435 219H0z`,
  rightharpoon: `M0 241v40h399993c4.7-4.7 7-9.3 7-14 0-9.3
-3.7-15.3-11-18-92.7-56.7-159-133.7-199-231-3.3-9.3-6-14.7-8-16-2-1.3-7-2-15-2
-10.7 0-16.7 2-18 6-2 2.7-1 9.7 3 21 15.3 42 36.7 81.8 64 119.5 27.3 37.7 58
 69.2 92 94.5zm0 0v40h399900v-40z`,
  rightharpoonplus: `M0 241v40h399993c4.7-4.7 7-9.3 7-14 0-9.3-3.7-15.3-11
-18-92.7-56.7-159-133.7-199-231-3.3-9.3-6-14.7-8-16-2-1.3-7-2-15-2-10.7 0-16.7
 2-18 6-2 2.7-1 9.7 3 21 15.3 42 36.7 81.8 64 119.5 27.3 37.7 58 69.2 92 94.5z
m0 0v40h399900v-40z m100 194v40h399900v-40zm0 0v40h399900v-40z`,
  rightharpoondown: `M399747 511c0 7.3 6.7 11 20 11 8 0 13-.8 15-2.5s4.7-6.8
 8-15.5c40-94 99.3-166.3 178-217 13.3-8 20.3-12.3 21-13 5.3-3.3 8.5-5.8 9.5
-7.5 1-1.7 1.5-5.2 1.5-10.5s-2.3-10.3-7-15H0v40h399908c-34 25.3-64.7 57-92 95
-27.3 38-48.7 77.7-64 119-3.3 8.7-5 14-5 16zM0 241v40h399900v-40z`,
  rightharpoondownplus: `M399747 705c0 7.3 6.7 11 20 11 8 0 13-.8
 15-2.5s4.7-6.8 8-15.5c40-94 99.3-166.3 178-217 13.3-8 20.3-12.3 21-13 5.3-3.3
 8.5-5.8 9.5-7.5 1-1.7 1.5-5.2 1.5-10.5s-2.3-10.3-7-15H0v40h399908c-34 25.3
-64.7 57-92 95-27.3 38-48.7 77.7-64 119-3.3 8.7-5 14-5 16zM0 435v40h399900v-40z
m0-194v40h400000v-40zm0 0v40h400000v-40z`,
  righthook: `M399859 241c-764 0 0 0 0 0 40-3.3 68.7-15.7 86-37 10-12 15-25.3
 15-40 0-22.7-9.8-40.7-29.5-54-19.7-13.3-43.5-21-71.5-23-17.3-1.3-26-8-26-20 0
-13.3 8.7-20 26-20 38 0 71 11.2 99 33.5 0 0 7 5.6 21 16.7 14 11.2 21 33.5 21
 66.8s-14 61.2-42 83.5c-28 22.3-61 33.5-99 33.5L0 241z M0 281v-40h399859v40z`,
  rightlinesegment: u0("M399960 241 V94 h40 V428 h-40 V281 H0 v-40z"),
  rightbracketunder: u0("M399995 0 h-120 V290 H0 v120 H400000z"),
  rightbracketover: u0("M399995 440 h-120 V150 H0 v-120 H399995z"),
  rightToFrom: `M400000 167c-70.7-42-118-97.7-142-167h-23c-15.3 0-23 .3-23
 1 0 1.3 5.3 13.7 16 37 18 35.3 41.3 69 70 101l7 8H0v40h399905l-7 8c-28.7 32
-52 65.7-70 101-10.7 23.3-16 35.7-16 37 0 .7 7.7 1 23 1h23c24-69.3 71.3-125 142
-167z M100 147v40h399900v-40zM0 341v40h399900v-40z`,
  // twoheadleftarrow is from glyph U+219E in font KaTeX AMS Regular
  twoheadleftarrow: `M0 167c68 40
 115.7 95.7 143 167h22c15.3 0 23-.3 23-1 0-1.3-5.3-13.7-16-37-18-35.3-41.3-69
-70-101l-7-8h125l9 7c50.7 39.3 85 86 103 140h46c0-4.7-6.3-18.7-19-42-18-35.3
-40-67.3-66-96l-9-9h399716v-40H284l9-9c26-28.7 48-60.7 66-96 12.7-23.333 19
-37.333 19-42h-46c-18 54-52.3 100.7-103 140l-9 7H95l7-8c28.7-32 52-65.7 70-101
 10.7-23.333 16-35.7 16-37 0-.7-7.7-1-23-1h-22C115.7 71.3 68 127 0 167z`,
  twoheadrightarrow: `M400000 167
c-68-40-115.7-95.7-143-167h-22c-15.3 0-23 .3-23 1 0 1.3 5.3 13.7 16 37 18 35.3
 41.3 69 70 101l7 8h-125l-9-7c-50.7-39.3-85-86-103-140h-46c0 4.7 6.3 18.7 19 42
 18 35.3 40 67.3 66 96l9 9H0v40h399716l-9 9c-26 28.7-48 60.7-66 96-12.7 23.333
-19 37.333-19 42h46c18-54 52.3-100.7 103-140l9-7h125l-7 8c-28.7 32-52 65.7-70
 101-10.7 23.333-16 35.7-16 37 0 .7 7.7 1 23 1h22c27.3-71.3 75-127 143-167z`,
  // tilde1 is a modified version of a glyph from the MnSymbol package
  tilde1: `M200 55.538c-77 0-168 73.953-177 73.953-3 0-7
-2.175-9-5.437L2 97c-1-2-2-4-2-6 0-4 2-7 5-9l20-12C116 12 171 0 207 0c86 0
 114 68 191 68 78 0 168-68 177-68 4 0 7 2 9 5l12 19c1 2.175 2 4.35 2 6.525 0
 4.35-2 7.613-5 9.788l-19 13.05c-92 63.077-116.937 75.308-183 76.128
-68.267.847-113-73.952-191-73.952z`,
  // ditto tilde2, tilde3, & tilde4
  tilde2: `M344 55.266c-142 0-300.638 81.316-311.5 86.418
-8.01 3.762-22.5 10.91-23.5 5.562L1 120c-1-2-1-3-1-4 0-5 3-9 8-10l18.4-9C160.9
 31.9 283 0 358 0c148 0 188 122 331 122s314-97 326-97c4 0 8 2 10 7l7 21.114
c1 2.14 1 3.21 1 4.28 0 5.347-3 9.626-7 10.696l-22.3 12.622C852.6 158.372 751
 181.476 676 181.476c-149 0-189-126.21-332-126.21z`,
  tilde3: `M786 59C457 59 32 175.242 13 175.242c-6 0-10-3.457
-11-10.37L.15 138c-1-7 3-12 10-13l19.2-6.4C378.4 40.7 634.3 0 804.3 0c337 0
 411.8 157 746.8 157 328 0 754-112 773-112 5 0 10 3 11 9l1 14.075c1 8.066-.697
 16.595-6.697 17.492l-21.052 7.31c-367.9 98.146-609.15 122.696-778.15 122.696
 -338 0-409-156.573-744-156.573z`,
  tilde4: `M786 58C457 58 32 177.487 13 177.487c-6 0-10-3.345
-11-10.035L.15 143c-1-7 3-12 10-13l22-6.7C381.2 35 637.15 0 807.15 0c337 0 409
 177 744 177 328 0 754-127 773-127 5 0 10 3 11 9l1 14.794c1 7.805-3 13.38-9
 14.495l-20.7 5.574c-366.85 99.79-607.3 139.372-776.3 139.372-338 0-409
 -175.236-744-175.236z`,
  // vec is from glyph U+20D7 in font KaTeX Main
  vec: `M377 20c0-5.333 1.833-10 5.5-14S391 0 397 0c4.667 0 8.667 1.667 12 5
3.333 2.667 6.667 9 10 19 6.667 24.667 20.333 43.667 41 57 7.333 4.667 11
10.667 11 18 0 6-1 10-3 12s-6.667 5-14 9c-28.667 14.667-53.667 35.667-75 63
-1.333 1.333-3.167 3.5-5.5 6.5s-4 4.833-5 5.5c-1 .667-2.5 1.333-4.5 2s-4.333 1
-7 1c-4.667 0-9.167-1.833-13.5-5.5S337 184 337 178c0-12.667 15.667-32.333 47-59
H213l-171-1c-8.667-6-13-12.333-13-19 0-4.667 4.333-11.333 13-20h359
c-16-25.333-24-45-24-59z`,
  // widehat1 is a modified version of a glyph from the MnSymbol package
  widehat1: `M529 0h5l519 115c5 1 9 5 9 10 0 1-1 2-1 3l-4 22
c-1 5-5 9-11 9h-2L532 67 19 159h-2c-5 0-9-4-11-9l-5-22c-1-6 2-12 8-13z`,
  // ditto widehat2, widehat3, & widehat4
  widehat2: `M1181 0h2l1171 176c6 0 10 5 10 11l-2 23c-1 6-5 10
-11 10h-1L1182 67 15 220h-1c-6 0-10-4-11-10l-2-23c-1-6 4-11 10-11z`,
  widehat3: `M1181 0h2l1171 236c6 0 10 5 10 11l-2 23c-1 6-5 10
-11 10h-1L1182 67 15 280h-1c-6 0-10-4-11-10l-2-23c-1-6 4-11 10-11z`,
  widehat4: `M1181 0h2l1171 296c6 0 10 5 10 11l-2 23c-1 6-5 10
-11 10h-1L1182 67 15 340h-1c-6 0-10-4-11-10l-2-23c-1-6 4-11 10-11z`,
  // widecheck paths are all inverted versions of widehat
  widecheck1: `M529,159h5l519,-115c5,-1,9,-5,9,-10c0,-1,-1,-2,-1,-3l-4,-22c-1,
-5,-5,-9,-11,-9h-2l-512,92l-513,-92h-2c-5,0,-9,4,-11,9l-5,22c-1,6,2,12,8,13z`,
  widecheck2: `M1181,220h2l1171,-176c6,0,10,-5,10,-11l-2,-23c-1,-6,-5,-10,
-11,-10h-1l-1168,153l-1167,-153h-1c-6,0,-10,4,-11,10l-2,23c-1,6,4,11,10,11z`,
  widecheck3: `M1181,280h2l1171,-236c6,0,10,-5,10,-11l-2,-23c-1,-6,-5,-10,
-11,-10h-1l-1168,213l-1167,-213h-1c-6,0,-10,4,-11,10l-2,23c-1,6,4,11,10,11z`,
  widecheck4: `M1181,340h2l1171,-296c6,0,10,-5,10,-11l-2,-23c-1,-6,-5,-10,
-11,-10h-1l-1168,273l-1167,-273h-1c-6,0,-10,4,-11,10l-2,23c-1,6,4,11,10,11z`,
  // The next ten paths support reaction arrows from the mhchem package.
  // Arrows for \ce{<-->} are offset from xAxis by 0.22ex, per mhchem in LaTeX
  // baraboveleftarrow is mostly from glyph U+2190 in font KaTeX Main
  baraboveleftarrow: `M400000 620h-399890l3 -3c68.7 -52.7 113.7 -120 135 -202
c4 -14.7 6 -23 6 -25c0 -7.3 -7 -11 -21 -11c-8 0 -13.2 0.8 -15.5 2.5
c-2.3 1.7 -4.2 5.8 -5.5 12.5c-1.3 4.7 -2.7 10.3 -4 17c-12 48.7 -34.8 92 -68.5 130
s-74.2 66.3 -121.5 85c-10 4 -16 7.7 -18 11c0 8.7 6 14.3 18 17c47.3 18.7 87.8 47
121.5 85s56.5 81.3 68.5 130c0.7 2 1.3 5 2 9s1.2 6.7 1.5 8c0.3 1.3 1 3.3 2 6
s2.2 4.5 3.5 5.5c1.3 1 3.3 1.8 6 2.5s6 1 10 1c14 0 21 -3.7 21 -11
c0 -2 -2 -10.3 -6 -25c-20 -79.3 -65 -146.7 -135 -202l-3 -3h399890z
M100 620v40h399900v-40z M0 241v40h399900v-40zM0 241v40h399900v-40z`,
  // rightarrowabovebar is mostly from glyph U+2192, KaTeX Main
  rightarrowabovebar: `M0 241v40h399891c-47.3 35.3-84 78-110 128-16.7 32
-27.7 63.7-33 95 0 1.3-.2 2.7-.5 4-.3 1.3-.5 2.3-.5 3 0 7.3 6.7 11 20 11 8 0
13.2-.8 15.5-2.5 2.3-1.7 4.2-5.5 5.5-11.5 2-13.3 5.7-27 11-41 14.7-44.7 39
-84.5 73-119.5s73.7-60.2 119-75.5c6-2 9-5.7 9-11s-3-9-9-11c-45.3-15.3-85-40.5
-119-75.5s-58.3-74.8-73-119.5c-4.7-14-8.3-27.3-11-40-1.3-6.7-3.2-10.8-5.5
-12.5-2.3-1.7-7.5-2.5-15.5-2.5-14 0-21 3.7-21 11 0 2 2 10.3 6 25 20.7 83.3 67
151.7 139 205zm96 379h399894v40H0zm0 0h399904v40H0z`,
  // The short left harpoon has 0.5em (i.e. 500 units) kern on the left end.
  // Ref from mhchem.sty: \rlap{\raisebox{-.22ex}{$\kern0.5em
  baraboveshortleftharpoon: `M507,435c-4,4,-6.3,8.7,-7,14c0,5.3,0.7,9,2,11
c1.3,2,5.3,5.3,12,10c90.7,54,156,130,196,228c3.3,10.7,6.3,16.3,9,17
c2,0.7,5,1,9,1c0,0,5,0,5,0c10.7,0,16.7,-2,18,-6c2,-2.7,1,-9.7,-3,-21
c-32,-87.3,-82.7,-157.7,-152,-211c0,0,-3,-3,-3,-3l399351,0l0,-40
c-398570,0,-399437,0,-399437,0z M593 435 v40 H399500 v-40z
M0 281 v-40 H399908 v40z M0 281 v-40 H399908 v40z`,
  rightharpoonaboveshortbar: `M0,241 l0,40c399126,0,399993,0,399993,0
c4.7,-4.7,7,-9.3,7,-14c0,-9.3,-3.7,-15.3,-11,-18c-92.7,-56.7,-159,-133.7,-199,
-231c-3.3,-9.3,-6,-14.7,-8,-16c-2,-1.3,-7,-2,-15,-2c-10.7,0,-16.7,2,-18,6
c-2,2.7,-1,9.7,3,21c15.3,42,36.7,81.8,64,119.5c27.3,37.7,58,69.2,92,94.5z
M0 241 v40 H399908 v-40z M0 475 v-40 H399500 v40z M0 475 v-40 H399500 v40z`,
  shortbaraboveleftharpoon: `M7,435c-4,4,-6.3,8.7,-7,14c0,5.3,0.7,9,2,11
c1.3,2,5.3,5.3,12,10c90.7,54,156,130,196,228c3.3,10.7,6.3,16.3,9,17c2,0.7,5,1,9,
1c0,0,5,0,5,0c10.7,0,16.7,-2,18,-6c2,-2.7,1,-9.7,-3,-21c-32,-87.3,-82.7,-157.7,
-152,-211c0,0,-3,-3,-3,-3l399907,0l0,-40c-399126,0,-399993,0,-399993,0z
M93 435 v40 H400000 v-40z M500 241 v40 H400000 v-40z M500 241 v40 H400000 v-40z`,
  shortrightharpoonabovebar: `M53,241l0,40c398570,0,399437,0,399437,0
c4.7,-4.7,7,-9.3,7,-14c0,-9.3,-3.7,-15.3,-11,-18c-92.7,-56.7,-159,-133.7,-199,
-231c-3.3,-9.3,-6,-14.7,-8,-16c-2,-1.3,-7,-2,-15,-2c-10.7,0,-16.7,2,-18,6
c-2,2.7,-1,9.7,3,21c15.3,42,36.7,81.8,64,119.5c27.3,37.7,58,69.2,92,94.5z
M500 241 v40 H399408 v-40z M500 435 v40 H400000 v-40z`
}, $n = function(e, t) {
  switch (e) {
    case "lbrack":
      return "M403 1759 V84 H666 V0 H319 V1759 v" + t + ` v1759 v84 h347 v-84
H403z M403 1759 V0 H319 V1759 v` + t + " v1759 v84 h84z";
    case "rbrack":
      return "M347 1759 V0 H0 V84 H263 V1759 v" + t + ` v1759 H0 v84 H347z
M347 1759 V0 H263 V1759 v` + t + " v1759 h84z";
    case "vert":
      return "M145 15 v585 v" + t + ` v585 c2.667,10,9.667,15,21,15
c10,0,16.667,-5,20,-15 v-585 v` + -t + ` v-585 c-2.667,-10,-9.667,-15,-21,-15
c-10,0,-16.667,5,-20,15z M188 15 H145 v585 v` + t + " v585 h43z";
    case "doublevert":
      return "M145 15 v585 v" + t + ` v585 c2.667,10,9.667,15,21,15
c10,0,16.667,-5,20,-15 v-585 v` + -t + ` v-585 c-2.667,-10,-9.667,-15,-21,-15
c-10,0,-16.667,5,-20,15z M188 15 H145 v585 v` + t + ` v585 h43z
M367 15 v585 v` + t + ` v585 c2.667,10,9.667,15,21,15
c10,0,16.667,-5,20,-15 v-585 v` + -t + ` v-585 c-2.667,-10,-9.667,-15,-21,-15
c-10,0,-16.667,5,-20,15z M410 15 H367 v585 v` + t + " v585 h43z";
    case "lfloor":
      return "M319 602 V0 H403 V602 v" + t + ` v1715 h263 v84 H319z
MM319 602 V0 H403 V602 v` + t + " v1715 H319z";
    case "rfloor":
      return "M319 602 V0 H403 V602 v" + t + ` v1799 H0 v-84 H319z
MM319 602 V0 H403 V602 v` + t + " v1715 H319z";
    case "lceil":
      return "M403 1759 V84 H666 V0 H319 V1759 v" + t + ` v602 h84z
M403 1759 V0 H319 V1759 v` + t + " v602 h84z";
    case "rceil":
      return "M347 1759 V0 H0 V84 H263 V1759 v" + t + ` v602 h84z
M347 1759 V0 h-84 V1759 v` + t + " v602 h84z";
    case "lparen":
      return `M863,9c0,-2,-2,-5,-6,-9c0,0,-17,0,-17,0c-12.7,0,-19.3,0.3,-20,1
c-5.3,5.3,-10.3,11,-15,17c-242.7,294.7,-395.3,682,-458,1162c-21.3,163.3,-33.3,349,
-36,557 l0,` + (t + 84) + `c0.2,6,0,26,0,60c2,159.3,10,310.7,24,454c53.3,528,210,
949.7,470,1265c4.7,6,9.7,11.7,15,17c0.7,0.7,7,1,19,1c0,0,18,0,18,0c4,-4,6,-7,6,-9
c0,-2.7,-3.3,-8.7,-10,-18c-135.3,-192.7,-235.5,-414.3,-300.5,-665c-65,-250.7,-102.5,
-544.7,-112.5,-882c-2,-104,-3,-167,-3,-189
l0,-` + (t + 92) + `c0,-162.7,5.7,-314,17,-454c20.7,-272,63.7,-513,129,-723c65.3,
-210,155.3,-396.3,270,-559c6.7,-9.3,10,-15.3,10,-18z`;
    case "rparen":
      return `M76,0c-16.7,0,-25,3,-25,9c0,2,2,6.3,6,13c21.3,28.7,42.3,60.3,
63,95c96.7,156.7,172.8,332.5,228.5,527.5c55.7,195,92.8,416.5,111.5,664.5
c11.3,139.3,17,290.7,17,454c0,28,1.7,43,3.3,45l0,` + (t + 9) + `
c-3,4,-3.3,16.7,-3.3,38c0,162,-5.7,313.7,-17,455c-18.7,248,-55.8,469.3,-111.5,664
c-55.7,194.7,-131.8,370.3,-228.5,527c-20.7,34.7,-41.7,66.3,-63,95c-2,3.3,-4,7,-6,11
c0,7.3,5.7,11,17,11c0,0,11,0,11,0c9.3,0,14.3,-0.3,15,-1c5.3,-5.3,10.3,-11,15,-17
c242.7,-294.7,395.3,-681.7,458,-1161c21.3,-164.7,33.3,-350.7,36,-558
l0,-` + (t + 144) + `c-2,-159.3,-10,-310.7,-24,-454c-53.3,-528,-210,-949.7,
-470,-1265c-4.7,-6,-9.7,-11.7,-15,-17c-0.7,-0.7,-6.7,-1,-18,-1z`;
    default:
      throw new Error("Unknown stretchy delimiter.");
  }
};
function Pn(r) {
  return "toText" in r;
}
class ge {
  // Never used; needed for satisfying interface.
  constructor(e) {
    this.children = void 0, this.classes = void 0, this.height = void 0, this.depth = void 0, this.maxFontSize = void 0, this.style = void 0, this.children = e, this.classes = [], this.height = 0, this.depth = 0, this.maxFontSize = 0, this.style = {};
  }
  hasClass(e) {
    return this.classes.includes(e);
  }
  /** Convert the fragment into a node. */
  toNode() {
    for (var e = document.createDocumentFragment(), t = 0; t < this.children.length; t++)
      e.appendChild(this.children[t].toNode());
    return e;
  }
  /** Convert the fragment into HTML markup. */
  toMarkup() {
    for (var e = "", t = 0; t < this.children.length; t++)
      e += this.children[t].toMarkup();
    return e;
  }
  /**
   * Converts the math node into a string, similar to innerText. Applies to
   * MathDomNode's only.
   */
  toText() {
    return this.children.map((e) => {
      if (Pn(e))
        return e.toText();
      throw new Error("Expected MathDomNode with toText, got " + e.constructor.name);
    }).join("");
  }
}
var Tt = {
  // https://en.wikibooks.org/wiki/LaTeX/Lengths and
  // https://tex.stackexchange.com/a/8263
  pt: 1,
  // TeX point
  mm: 7227 / 2540,
  // millimeter
  cm: 7227 / 254,
  // centimeter
  in: 72.27,
  // inch
  bp: 803 / 800,
  // big (PostScript) points
  pc: 12,
  // pica
  dd: 1238 / 1157,
  // didot
  cc: 14856 / 1157,
  // cicero (12 didot)
  nd: 685 / 642,
  // new didot
  nc: 1370 / 107,
  // new cicero (12 new didot)
  sp: 1 / 65536,
  // scaled point (TeX's internal smallest unit)
  // https://tex.stackexchange.com/a/41371
  px: 803 / 800
  // \pdfpxdimen defaults to 1 bp in pdfTeX and LuaTeX
}, Gn = {
  ex: !0,
  em: !0,
  mu: !0
}, Kr = function(e) {
  return typeof e != "string" && (e = e.unit), e in Tt || e in Gn || e === "ex";
}, a0 = function(e, t) {
  var a;
  if (e.unit in Tt)
    a = Tt[e.unit] / t.fontMetrics().ptPerEm / t.sizeMultiplier;
  else if (e.unit === "mu")
    a = t.fontMetrics().cssEmPerMu;
  else {
    var n;
    if (t.style.isTight() ? n = t.havingStyle(t.style.text()) : n = t, e.unit === "ex")
      a = n.fontMetrics().xHeight;
    else if (e.unit === "em")
      a = n.fontMetrics().quad;
    else
      throw new E("Invalid unit: '" + e.unit + "'");
    n !== t && (a *= n.sizeMultiplier / t.sizeMultiplier);
  }
  return Math.min(e.number * a, t.maxSize);
}, B = function(e) {
  return +e.toFixed(4) + "em";
}, Q0 = function(e) {
  return e.filter((t) => t).join(" ");
}, jt = function(e) {
  var t = "";
  for (var a of Object.keys(e)) {
    var n = e[a];
    n !== void 0 && (t += gn(a) + ":" + n + ";");
  }
  return t;
}, Jr = function(e, t, a) {
  if (this.classes = e || [], this.attributes = {}, this.height = 0, this.depth = 0, this.maxFontSize = 0, this.style = a || {}, t) {
    t.style.isTight() && this.classes.push("mtight");
    var n = t.getColor();
    n && (this.style.color = n);
  }
}, Qr = function(e) {
  var t = document.createElement(e);
  t.className = Q0(this.classes), Object.assign(t.style, this.style);
  for (var a of Object.keys(this.attributes))
    t.setAttribute(a, this.attributes[a]);
  for (var n = 0; n < this.children.length; n++)
    t.appendChild(this.children[n].toNode());
  return t;
}, Un = /[\s"'>/=\x00-\x1f]/, ea = function(e) {
  var t = "<" + e;
  this.classes.length && (t += ' class="' + d0(Q0(this.classes)) + '"');
  var a = jt(this.style);
  a && (t += ' style="' + d0(a) + '"');
  for (var n of Object.keys(this.attributes)) {
    if (Un.test(n))
      throw new E("Invalid attribute name '" + n + "'");
    t += " " + n + '="' + d0(this.attributes[n]) + '"';
  }
  t += ">";
  for (var s = 0; s < this.children.length; s++)
    t += this.children[s].toMarkup();
  return t += "</" + e + ">", t;
};
class be {
  constructor(e, t, a, n) {
    this.children = void 0, this.attributes = void 0, this.classes = void 0, this.height = void 0, this.depth = void 0, this.width = void 0, this.maxFontSize = void 0, this.style = void 0, this.italic = void 0, Jr.call(this, e, a, n), this.children = t || [];
  }
  /**
   * Sets an arbitrary attribute on the span. Warning: use this wisely. Not
   * all browsers support attributes the same, and having too many custom
   * attributes is probably bad.
   */
  setAttribute(e, t) {
    this.attributes[e] = t;
  }
  hasClass(e) {
    return this.classes.includes(e);
  }
  toNode() {
    return Qr.call(this, "span");
  }
  toMarkup() {
    return ea.call(this, "span");
  }
}
class Xe {
  constructor(e, t, a, n) {
    this.children = void 0, this.attributes = void 0, this.classes = void 0, this.height = void 0, this.depth = void 0, this.maxFontSize = void 0, this.style = void 0, Jr.call(this, t, n), this.children = a || [], this.setAttribute("href", e);
  }
  setAttribute(e, t) {
    this.attributes[e] = t;
  }
  hasClass(e) {
    return this.classes.includes(e);
  }
  toNode() {
    return Qr.call(this, "a");
  }
  toMarkup() {
    return ea.call(this, "a");
  }
}
class jn {
  constructor(e, t, a) {
    this.src = void 0, this.alt = void 0, this.classes = void 0, this.height = void 0, this.depth = void 0, this.maxFontSize = void 0, this.style = void 0, this.alt = t, this.src = e, this.classes = ["mord"], this.height = 0, this.depth = 0, this.maxFontSize = 0, this.style = a;
  }
  hasClass(e) {
    return this.classes.includes(e);
  }
  toNode() {
    var e = document.createElement("img");
    return e.src = this.src, e.alt = this.alt, e.className = "mord", Object.assign(e.style, this.style), e;
  }
  toMarkup() {
    var e = '<img src="' + d0(this.src) + '"' + (' alt="' + d0(this.alt) + '"'), t = jt(this.style);
    return t && (e += ' style="' + d0(t) + '"'), e += "'/>", e;
  }
}
var Xn = {
  î: "ı̂",
  ï: "ı̈",
  í: "ı́",
  // 'ī': '\u0131\u0304', // enable when we add Extended Latin
  ì: "ı̀"
};
class z0 {
  constructor(e, t, a, n, s, u, o, h) {
    this.text = void 0, this.height = void 0, this.depth = void 0, this.italic = void 0, this.skew = void 0, this.width = void 0, this.maxFontSize = void 0, this.classes = void 0, this.style = void 0, this.text = e, this.height = t || 0, this.depth = a || 0, this.italic = n || 0, this.skew = s || 0, this.width = u || 0, this.classes = o || [], this.style = h || {}, this.maxFontSize = 0;
    var m = En(this.text.charCodeAt(0));
    m && this.classes.push(m + "_fallback"), /[îïíì]/.test(this.text) && (this.text = Xn[this.text]);
  }
  hasClass(e) {
    return this.classes.includes(e);
  }
  /**
   * Creates a text node or span from a symbol node. Note that a span is only
   * created if it is needed.
   */
  toNode() {
    var e = document.createTextNode(this.text), t = null;
    return this.italic > 0 && (t = document.createElement("span"), t.style.marginRight = B(this.italic)), this.classes.length > 0 && (t = t || document.createElement("span"), t.className = Q0(this.classes)), Object.keys(this.style).length > 0 && (t = t || document.createElement("span"), Object.assign(t.style, this.style)), t ? (t.appendChild(e), t) : e;
  }
  /**
   * Creates markup for a symbol node.
   */
  toMarkup() {
    var e = !1, t = "<span";
    this.classes.length && (e = !0, t += ' class="', t += d0(Q0(this.classes)), t += '"');
    var a = "";
    this.italic > 0 && (a += "margin-right:" + B(this.italic) + ";"), a += jt(this.style), a && (e = !0, t += ' style="' + d0(a) + '"');
    var n = d0(this.text);
    return e ? (t += ">", t += n, t += "</span>", t) : n;
  }
}
class G0 {
  constructor(e, t) {
    this.children = void 0, this.attributes = void 0, this.children = e || [], this.attributes = t || {};
  }
  toNode() {
    var e = "http://www.w3.org/2000/svg", t = document.createElementNS(e, "svg");
    for (var a of Object.keys(this.attributes))
      t.setAttribute(a, this.attributes[a]);
    for (var n = 0; n < this.children.length; n++)
      t.appendChild(this.children[n].toNode());
    return t;
  }
  toMarkup() {
    var e = '<svg xmlns="http://www.w3.org/2000/svg"';
    for (var t of Object.keys(this.attributes))
      e += " " + t + '="' + d0(this.attributes[t]) + '"';
    e += ">";
    for (var a = 0; a < this.children.length; a++)
      e += this.children[a].toMarkup();
    return e += "</svg>", e;
  }
}
class ee {
  constructor(e, t) {
    this.pathName = void 0, this.alternate = void 0, this.pathName = e, this.alternate = t;
  }
  toNode() {
    var e = "http://www.w3.org/2000/svg", t = document.createElementNS(e, "path");
    return this.alternate ? t.setAttribute("d", this.alternate) : t.setAttribute("d", or[this.pathName]), t;
  }
  toMarkup() {
    return this.alternate ? '<path d="' + d0(this.alternate) + '"/>' : '<path d="' + d0(or[this.pathName]) + '"/>';
  }
}
class zt {
  constructor(e) {
    this.attributes = void 0, this.attributes = e || {};
  }
  toNode() {
    var e = "http://www.w3.org/2000/svg", t = document.createElementNS(e, "line");
    for (var a of Object.keys(this.attributes))
      t.setAttribute(a, this.attributes[a]);
    return t;
  }
  toMarkup() {
    var e = "<line";
    for (var t of Object.keys(this.attributes))
      e += " " + t + '="' + d0(this.attributes[t]) + '"';
    return e += "/>", e;
  }
}
function Vn(r) {
  if (r instanceof z0)
    return r;
  throw new Error("Expected symbolNode but got " + String(r) + ".");
}
function Yn(r) {
  if (r instanceof be)
    return r;
  throw new Error("Expected span<HtmlDomNode> but got " + String(r) + ".");
}
var Wn = (r) => r instanceof be || r instanceof Xe || r instanceof ge, I0 = {
  "AMS-Regular": {
    32: [0, 0, 0, 0, 0.25],
    65: [0, 0.68889, 0, 0, 0.72222],
    66: [0, 0.68889, 0, 0, 0.66667],
    67: [0, 0.68889, 0, 0, 0.72222],
    68: [0, 0.68889, 0, 0, 0.72222],
    69: [0, 0.68889, 0, 0, 0.66667],
    70: [0, 0.68889, 0, 0, 0.61111],
    71: [0, 0.68889, 0, 0, 0.77778],
    72: [0, 0.68889, 0, 0, 0.77778],
    73: [0, 0.68889, 0, 0, 0.38889],
    74: [0.16667, 0.68889, 0, 0, 0.5],
    75: [0, 0.68889, 0, 0, 0.77778],
    76: [0, 0.68889, 0, 0, 0.66667],
    77: [0, 0.68889, 0, 0, 0.94445],
    78: [0, 0.68889, 0, 0, 0.72222],
    79: [0.16667, 0.68889, 0, 0, 0.77778],
    80: [0, 0.68889, 0, 0, 0.61111],
    81: [0.16667, 0.68889, 0, 0, 0.77778],
    82: [0, 0.68889, 0, 0, 0.72222],
    83: [0, 0.68889, 0, 0, 0.55556],
    84: [0, 0.68889, 0, 0, 0.66667],
    85: [0, 0.68889, 0, 0, 0.72222],
    86: [0, 0.68889, 0, 0, 0.72222],
    87: [0, 0.68889, 0, 0, 1],
    88: [0, 0.68889, 0, 0, 0.72222],
    89: [0, 0.68889, 0, 0, 0.72222],
    90: [0, 0.68889, 0, 0, 0.66667],
    107: [0, 0.68889, 0, 0, 0.55556],
    160: [0, 0, 0, 0, 0.25],
    165: [0, 0.675, 0.025, 0, 0.75],
    174: [0.15559, 0.69224, 0, 0, 0.94666],
    240: [0, 0.68889, 0, 0, 0.55556],
    295: [0, 0.68889, 0, 0, 0.54028],
    710: [0, 0.825, 0, 0, 2.33334],
    732: [0, 0.9, 0, 0, 2.33334],
    770: [0, 0.825, 0, 0, 2.33334],
    771: [0, 0.9, 0, 0, 2.33334],
    989: [0.08167, 0.58167, 0, 0, 0.77778],
    1008: [0, 0.43056, 0.04028, 0, 0.66667],
    8245: [0, 0.54986, 0, 0, 0.275],
    8463: [0, 0.68889, 0, 0, 0.54028],
    8487: [0, 0.68889, 0, 0, 0.72222],
    8498: [0, 0.68889, 0, 0, 0.55556],
    8502: [0, 0.68889, 0, 0, 0.66667],
    8503: [0, 0.68889, 0, 0, 0.44445],
    8504: [0, 0.68889, 0, 0, 0.66667],
    8513: [0, 0.68889, 0, 0, 0.63889],
    8592: [-0.03598, 0.46402, 0, 0, 0.5],
    8594: [-0.03598, 0.46402, 0, 0, 0.5],
    8602: [-0.13313, 0.36687, 0, 0, 1],
    8603: [-0.13313, 0.36687, 0, 0, 1],
    8606: [0.01354, 0.52239, 0, 0, 1],
    8608: [0.01354, 0.52239, 0, 0, 1],
    8610: [0.01354, 0.52239, 0, 0, 1.11111],
    8611: [0.01354, 0.52239, 0, 0, 1.11111],
    8619: [0, 0.54986, 0, 0, 1],
    8620: [0, 0.54986, 0, 0, 1],
    8621: [-0.13313, 0.37788, 0, 0, 1.38889],
    8622: [-0.13313, 0.36687, 0, 0, 1],
    8624: [0, 0.69224, 0, 0, 0.5],
    8625: [0, 0.69224, 0, 0, 0.5],
    8630: [0, 0.43056, 0, 0, 1],
    8631: [0, 0.43056, 0, 0, 1],
    8634: [0.08198, 0.58198, 0, 0, 0.77778],
    8635: [0.08198, 0.58198, 0, 0, 0.77778],
    8638: [0.19444, 0.69224, 0, 0, 0.41667],
    8639: [0.19444, 0.69224, 0, 0, 0.41667],
    8642: [0.19444, 0.69224, 0, 0, 0.41667],
    8643: [0.19444, 0.69224, 0, 0, 0.41667],
    8644: [0.1808, 0.675, 0, 0, 1],
    8646: [0.1808, 0.675, 0, 0, 1],
    8647: [0.1808, 0.675, 0, 0, 1],
    8648: [0.19444, 0.69224, 0, 0, 0.83334],
    8649: [0.1808, 0.675, 0, 0, 1],
    8650: [0.19444, 0.69224, 0, 0, 0.83334],
    8651: [0.01354, 0.52239, 0, 0, 1],
    8652: [0.01354, 0.52239, 0, 0, 1],
    8653: [-0.13313, 0.36687, 0, 0, 1],
    8654: [-0.13313, 0.36687, 0, 0, 1],
    8655: [-0.13313, 0.36687, 0, 0, 1],
    8666: [0.13667, 0.63667, 0, 0, 1],
    8667: [0.13667, 0.63667, 0, 0, 1],
    8669: [-0.13313, 0.37788, 0, 0, 1],
    8672: [-0.064, 0.437, 0, 0, 1.334],
    8674: [-0.064, 0.437, 0, 0, 1.334],
    8705: [0, 0.825, 0, 0, 0.5],
    8708: [0, 0.68889, 0, 0, 0.55556],
    8709: [0.08167, 0.58167, 0, 0, 0.77778],
    8717: [0, 0.43056, 0, 0, 0.42917],
    8722: [-0.03598, 0.46402, 0, 0, 0.5],
    8724: [0.08198, 0.69224, 0, 0, 0.77778],
    8726: [0.08167, 0.58167, 0, 0, 0.77778],
    8733: [0, 0.69224, 0, 0, 0.77778],
    8736: [0, 0.69224, 0, 0, 0.72222],
    8737: [0, 0.69224, 0, 0, 0.72222],
    8738: [0.03517, 0.52239, 0, 0, 0.72222],
    8739: [0.08167, 0.58167, 0, 0, 0.22222],
    8740: [0.25142, 0.74111, 0, 0, 0.27778],
    8741: [0.08167, 0.58167, 0, 0, 0.38889],
    8742: [0.25142, 0.74111, 0, 0, 0.5],
    8756: [0, 0.69224, 0, 0, 0.66667],
    8757: [0, 0.69224, 0, 0, 0.66667],
    8764: [-0.13313, 0.36687, 0, 0, 0.77778],
    8765: [-0.13313, 0.37788, 0, 0, 0.77778],
    8769: [-0.13313, 0.36687, 0, 0, 0.77778],
    8770: [-0.03625, 0.46375, 0, 0, 0.77778],
    8774: [0.30274, 0.79383, 0, 0, 0.77778],
    8776: [-0.01688, 0.48312, 0, 0, 0.77778],
    8778: [0.08167, 0.58167, 0, 0, 0.77778],
    8782: [0.06062, 0.54986, 0, 0, 0.77778],
    8783: [0.06062, 0.54986, 0, 0, 0.77778],
    8785: [0.08198, 0.58198, 0, 0, 0.77778],
    8786: [0.08198, 0.58198, 0, 0, 0.77778],
    8787: [0.08198, 0.58198, 0, 0, 0.77778],
    8790: [0, 0.69224, 0, 0, 0.77778],
    8791: [0.22958, 0.72958, 0, 0, 0.77778],
    8796: [0.08198, 0.91667, 0, 0, 0.77778],
    8806: [0.25583, 0.75583, 0, 0, 0.77778],
    8807: [0.25583, 0.75583, 0, 0, 0.77778],
    8808: [0.25142, 0.75726, 0, 0, 0.77778],
    8809: [0.25142, 0.75726, 0, 0, 0.77778],
    8812: [0.25583, 0.75583, 0, 0, 0.5],
    8814: [0.20576, 0.70576, 0, 0, 0.77778],
    8815: [0.20576, 0.70576, 0, 0, 0.77778],
    8816: [0.30274, 0.79383, 0, 0, 0.77778],
    8817: [0.30274, 0.79383, 0, 0, 0.77778],
    8818: [0.22958, 0.72958, 0, 0, 0.77778],
    8819: [0.22958, 0.72958, 0, 0, 0.77778],
    8822: [0.1808, 0.675, 0, 0, 0.77778],
    8823: [0.1808, 0.675, 0, 0, 0.77778],
    8828: [0.13667, 0.63667, 0, 0, 0.77778],
    8829: [0.13667, 0.63667, 0, 0, 0.77778],
    8830: [0.22958, 0.72958, 0, 0, 0.77778],
    8831: [0.22958, 0.72958, 0, 0, 0.77778],
    8832: [0.20576, 0.70576, 0, 0, 0.77778],
    8833: [0.20576, 0.70576, 0, 0, 0.77778],
    8840: [0.30274, 0.79383, 0, 0, 0.77778],
    8841: [0.30274, 0.79383, 0, 0, 0.77778],
    8842: [0.13597, 0.63597, 0, 0, 0.77778],
    8843: [0.13597, 0.63597, 0, 0, 0.77778],
    8847: [0.03517, 0.54986, 0, 0, 0.77778],
    8848: [0.03517, 0.54986, 0, 0, 0.77778],
    8858: [0.08198, 0.58198, 0, 0, 0.77778],
    8859: [0.08198, 0.58198, 0, 0, 0.77778],
    8861: [0.08198, 0.58198, 0, 0, 0.77778],
    8862: [0, 0.675, 0, 0, 0.77778],
    8863: [0, 0.675, 0, 0, 0.77778],
    8864: [0, 0.675, 0, 0, 0.77778],
    8865: [0, 0.675, 0, 0, 0.77778],
    8872: [0, 0.69224, 0, 0, 0.61111],
    8873: [0, 0.69224, 0, 0, 0.72222],
    8874: [0, 0.69224, 0, 0, 0.88889],
    8876: [0, 0.68889, 0, 0, 0.61111],
    8877: [0, 0.68889, 0, 0, 0.61111],
    8878: [0, 0.68889, 0, 0, 0.72222],
    8879: [0, 0.68889, 0, 0, 0.72222],
    8882: [0.03517, 0.54986, 0, 0, 0.77778],
    8883: [0.03517, 0.54986, 0, 0, 0.77778],
    8884: [0.13667, 0.63667, 0, 0, 0.77778],
    8885: [0.13667, 0.63667, 0, 0, 0.77778],
    8888: [0, 0.54986, 0, 0, 1.11111],
    8890: [0.19444, 0.43056, 0, 0, 0.55556],
    8891: [0.19444, 0.69224, 0, 0, 0.61111],
    8892: [0.19444, 0.69224, 0, 0, 0.61111],
    8901: [0, 0.54986, 0, 0, 0.27778],
    8903: [0.08167, 0.58167, 0, 0, 0.77778],
    8905: [0.08167, 0.58167, 0, 0, 0.77778],
    8906: [0.08167, 0.58167, 0, 0, 0.77778],
    8907: [0, 0.69224, 0, 0, 0.77778],
    8908: [0, 0.69224, 0, 0, 0.77778],
    8909: [-0.03598, 0.46402, 0, 0, 0.77778],
    8910: [0, 0.54986, 0, 0, 0.76042],
    8911: [0, 0.54986, 0, 0, 0.76042],
    8912: [0.03517, 0.54986, 0, 0, 0.77778],
    8913: [0.03517, 0.54986, 0, 0, 0.77778],
    8914: [0, 0.54986, 0, 0, 0.66667],
    8915: [0, 0.54986, 0, 0, 0.66667],
    8916: [0, 0.69224, 0, 0, 0.66667],
    8918: [0.0391, 0.5391, 0, 0, 0.77778],
    8919: [0.0391, 0.5391, 0, 0, 0.77778],
    8920: [0.03517, 0.54986, 0, 0, 1.33334],
    8921: [0.03517, 0.54986, 0, 0, 1.33334],
    8922: [0.38569, 0.88569, 0, 0, 0.77778],
    8923: [0.38569, 0.88569, 0, 0, 0.77778],
    8926: [0.13667, 0.63667, 0, 0, 0.77778],
    8927: [0.13667, 0.63667, 0, 0, 0.77778],
    8928: [0.30274, 0.79383, 0, 0, 0.77778],
    8929: [0.30274, 0.79383, 0, 0, 0.77778],
    8934: [0.23222, 0.74111, 0, 0, 0.77778],
    8935: [0.23222, 0.74111, 0, 0, 0.77778],
    8936: [0.23222, 0.74111, 0, 0, 0.77778],
    8937: [0.23222, 0.74111, 0, 0, 0.77778],
    8938: [0.20576, 0.70576, 0, 0, 0.77778],
    8939: [0.20576, 0.70576, 0, 0, 0.77778],
    8940: [0.30274, 0.79383, 0, 0, 0.77778],
    8941: [0.30274, 0.79383, 0, 0, 0.77778],
    8994: [0.19444, 0.69224, 0, 0, 0.77778],
    8995: [0.19444, 0.69224, 0, 0, 0.77778],
    9416: [0.15559, 0.69224, 0, 0, 0.90222],
    9484: [0, 0.69224, 0, 0, 0.5],
    9488: [0, 0.69224, 0, 0, 0.5],
    9492: [0, 0.37788, 0, 0, 0.5],
    9496: [0, 0.37788, 0, 0, 0.5],
    9585: [0.19444, 0.68889, 0, 0, 0.88889],
    9586: [0.19444, 0.74111, 0, 0, 0.88889],
    9632: [0, 0.675, 0, 0, 0.77778],
    9633: [0, 0.675, 0, 0, 0.77778],
    9650: [0, 0.54986, 0, 0, 0.72222],
    9651: [0, 0.54986, 0, 0, 0.72222],
    9654: [0.03517, 0.54986, 0, 0, 0.77778],
    9660: [0, 0.54986, 0, 0, 0.72222],
    9661: [0, 0.54986, 0, 0, 0.72222],
    9664: [0.03517, 0.54986, 0, 0, 0.77778],
    9674: [0.11111, 0.69224, 0, 0, 0.66667],
    9733: [0.19444, 0.69224, 0, 0, 0.94445],
    10003: [0, 0.69224, 0, 0, 0.83334],
    10016: [0, 0.69224, 0, 0, 0.83334],
    10731: [0.11111, 0.69224, 0, 0, 0.66667],
    10846: [0.19444, 0.75583, 0, 0, 0.61111],
    10877: [0.13667, 0.63667, 0, 0, 0.77778],
    10878: [0.13667, 0.63667, 0, 0, 0.77778],
    10885: [0.25583, 0.75583, 0, 0, 0.77778],
    10886: [0.25583, 0.75583, 0, 0, 0.77778],
    10887: [0.13597, 0.63597, 0, 0, 0.77778],
    10888: [0.13597, 0.63597, 0, 0, 0.77778],
    10889: [0.26167, 0.75726, 0, 0, 0.77778],
    10890: [0.26167, 0.75726, 0, 0, 0.77778],
    10891: [0.48256, 0.98256, 0, 0, 0.77778],
    10892: [0.48256, 0.98256, 0, 0, 0.77778],
    10901: [0.13667, 0.63667, 0, 0, 0.77778],
    10902: [0.13667, 0.63667, 0, 0, 0.77778],
    10933: [0.25142, 0.75726, 0, 0, 0.77778],
    10934: [0.25142, 0.75726, 0, 0, 0.77778],
    10935: [0.26167, 0.75726, 0, 0, 0.77778],
    10936: [0.26167, 0.75726, 0, 0, 0.77778],
    10937: [0.26167, 0.75726, 0, 0, 0.77778],
    10938: [0.26167, 0.75726, 0, 0, 0.77778],
    10949: [0.25583, 0.75583, 0, 0, 0.77778],
    10950: [0.25583, 0.75583, 0, 0, 0.77778],
    10955: [0.28481, 0.79383, 0, 0, 0.77778],
    10956: [0.28481, 0.79383, 0, 0, 0.77778],
    57350: [0.08167, 0.58167, 0, 0, 0.22222],
    57351: [0.08167, 0.58167, 0, 0, 0.38889],
    57352: [0.08167, 0.58167, 0, 0, 0.77778],
    57353: [0, 0.43056, 0.04028, 0, 0.66667],
    57356: [0.25142, 0.75726, 0, 0, 0.77778],
    57357: [0.25142, 0.75726, 0, 0, 0.77778],
    57358: [0.41951, 0.91951, 0, 0, 0.77778],
    57359: [0.30274, 0.79383, 0, 0, 0.77778],
    57360: [0.30274, 0.79383, 0, 0, 0.77778],
    57361: [0.41951, 0.91951, 0, 0, 0.77778],
    57366: [0.25142, 0.75726, 0, 0, 0.77778],
    57367: [0.25142, 0.75726, 0, 0, 0.77778],
    57368: [0.25142, 0.75726, 0, 0, 0.77778],
    57369: [0.25142, 0.75726, 0, 0, 0.77778],
    57370: [0.13597, 0.63597, 0, 0, 0.77778],
    57371: [0.13597, 0.63597, 0, 0, 0.77778]
  },
  "Caligraphic-Regular": {
    32: [0, 0, 0, 0, 0.25],
    65: [0, 0.68333, 0, 0.19445, 0.79847],
    66: [0, 0.68333, 0.03041, 0.13889, 0.65681],
    67: [0, 0.68333, 0.05834, 0.13889, 0.52653],
    68: [0, 0.68333, 0.02778, 0.08334, 0.77139],
    69: [0, 0.68333, 0.08944, 0.11111, 0.52778],
    70: [0, 0.68333, 0.09931, 0.11111, 0.71875],
    71: [0.09722, 0.68333, 0.0593, 0.11111, 0.59487],
    72: [0, 0.68333, 965e-5, 0.11111, 0.84452],
    73: [0, 0.68333, 0.07382, 0, 0.54452],
    74: [0.09722, 0.68333, 0.18472, 0.16667, 0.67778],
    75: [0, 0.68333, 0.01445, 0.05556, 0.76195],
    76: [0, 0.68333, 0, 0.13889, 0.68972],
    77: [0, 0.68333, 0, 0.13889, 1.2009],
    78: [0, 0.68333, 0.14736, 0.08334, 0.82049],
    79: [0, 0.68333, 0.02778, 0.11111, 0.79611],
    80: [0, 0.68333, 0.08222, 0.08334, 0.69556],
    81: [0.09722, 0.68333, 0, 0.11111, 0.81667],
    82: [0, 0.68333, 0, 0.08334, 0.8475],
    83: [0, 0.68333, 0.075, 0.13889, 0.60556],
    84: [0, 0.68333, 0.25417, 0, 0.54464],
    85: [0, 0.68333, 0.09931, 0.08334, 0.62583],
    86: [0, 0.68333, 0.08222, 0, 0.61278],
    87: [0, 0.68333, 0.08222, 0.08334, 0.98778],
    88: [0, 0.68333, 0.14643, 0.13889, 0.7133],
    89: [0.09722, 0.68333, 0.08222, 0.08334, 0.66834],
    90: [0, 0.68333, 0.07944, 0.13889, 0.72473],
    160: [0, 0, 0, 0, 0.25]
  },
  "Fraktur-Regular": {
    32: [0, 0, 0, 0, 0.25],
    33: [0, 0.69141, 0, 0, 0.29574],
    34: [0, 0.69141, 0, 0, 0.21471],
    38: [0, 0.69141, 0, 0, 0.73786],
    39: [0, 0.69141, 0, 0, 0.21201],
    40: [0.24982, 0.74947, 0, 0, 0.38865],
    41: [0.24982, 0.74947, 0, 0, 0.38865],
    42: [0, 0.62119, 0, 0, 0.27764],
    43: [0.08319, 0.58283, 0, 0, 0.75623],
    44: [0, 0.10803, 0, 0, 0.27764],
    45: [0.08319, 0.58283, 0, 0, 0.75623],
    46: [0, 0.10803, 0, 0, 0.27764],
    47: [0.24982, 0.74947, 0, 0, 0.50181],
    48: [0, 0.47534, 0, 0, 0.50181],
    49: [0, 0.47534, 0, 0, 0.50181],
    50: [0, 0.47534, 0, 0, 0.50181],
    51: [0.18906, 0.47534, 0, 0, 0.50181],
    52: [0.18906, 0.47534, 0, 0, 0.50181],
    53: [0.18906, 0.47534, 0, 0, 0.50181],
    54: [0, 0.69141, 0, 0, 0.50181],
    55: [0.18906, 0.47534, 0, 0, 0.50181],
    56: [0, 0.69141, 0, 0, 0.50181],
    57: [0.18906, 0.47534, 0, 0, 0.50181],
    58: [0, 0.47534, 0, 0, 0.21606],
    59: [0.12604, 0.47534, 0, 0, 0.21606],
    61: [-0.13099, 0.36866, 0, 0, 0.75623],
    63: [0, 0.69141, 0, 0, 0.36245],
    65: [0, 0.69141, 0, 0, 0.7176],
    66: [0, 0.69141, 0, 0, 0.88397],
    67: [0, 0.69141, 0, 0, 0.61254],
    68: [0, 0.69141, 0, 0, 0.83158],
    69: [0, 0.69141, 0, 0, 0.66278],
    70: [0.12604, 0.69141, 0, 0, 0.61119],
    71: [0, 0.69141, 0, 0, 0.78539],
    72: [0.06302, 0.69141, 0, 0, 0.7203],
    73: [0, 0.69141, 0, 0, 0.55448],
    74: [0.12604, 0.69141, 0, 0, 0.55231],
    75: [0, 0.69141, 0, 0, 0.66845],
    76: [0, 0.69141, 0, 0, 0.66602],
    77: [0, 0.69141, 0, 0, 1.04953],
    78: [0, 0.69141, 0, 0, 0.83212],
    79: [0, 0.69141, 0, 0, 0.82699],
    80: [0.18906, 0.69141, 0, 0, 0.82753],
    81: [0.03781, 0.69141, 0, 0, 0.82699],
    82: [0, 0.69141, 0, 0, 0.82807],
    83: [0, 0.69141, 0, 0, 0.82861],
    84: [0, 0.69141, 0, 0, 0.66899],
    85: [0, 0.69141, 0, 0, 0.64576],
    86: [0, 0.69141, 0, 0, 0.83131],
    87: [0, 0.69141, 0, 0, 1.04602],
    88: [0, 0.69141, 0, 0, 0.71922],
    89: [0.18906, 0.69141, 0, 0, 0.83293],
    90: [0.12604, 0.69141, 0, 0, 0.60201],
    91: [0.24982, 0.74947, 0, 0, 0.27764],
    93: [0.24982, 0.74947, 0, 0, 0.27764],
    94: [0, 0.69141, 0, 0, 0.49965],
    97: [0, 0.47534, 0, 0, 0.50046],
    98: [0, 0.69141, 0, 0, 0.51315],
    99: [0, 0.47534, 0, 0, 0.38946],
    100: [0, 0.62119, 0, 0, 0.49857],
    101: [0, 0.47534, 0, 0, 0.40053],
    102: [0.18906, 0.69141, 0, 0, 0.32626],
    103: [0.18906, 0.47534, 0, 0, 0.5037],
    104: [0.18906, 0.69141, 0, 0, 0.52126],
    105: [0, 0.69141, 0, 0, 0.27899],
    106: [0, 0.69141, 0, 0, 0.28088],
    107: [0, 0.69141, 0, 0, 0.38946],
    108: [0, 0.69141, 0, 0, 0.27953],
    109: [0, 0.47534, 0, 0, 0.76676],
    110: [0, 0.47534, 0, 0, 0.52666],
    111: [0, 0.47534, 0, 0, 0.48885],
    112: [0.18906, 0.52396, 0, 0, 0.50046],
    113: [0.18906, 0.47534, 0, 0, 0.48912],
    114: [0, 0.47534, 0, 0, 0.38919],
    115: [0, 0.47534, 0, 0, 0.44266],
    116: [0, 0.62119, 0, 0, 0.33301],
    117: [0, 0.47534, 0, 0, 0.5172],
    118: [0, 0.52396, 0, 0, 0.5118],
    119: [0, 0.52396, 0, 0, 0.77351],
    120: [0.18906, 0.47534, 0, 0, 0.38865],
    121: [0.18906, 0.47534, 0, 0, 0.49884],
    122: [0.18906, 0.47534, 0, 0, 0.39054],
    160: [0, 0, 0, 0, 0.25],
    8216: [0, 0.69141, 0, 0, 0.21471],
    8217: [0, 0.69141, 0, 0, 0.21471],
    58112: [0, 0.62119, 0, 0, 0.49749],
    58113: [0, 0.62119, 0, 0, 0.4983],
    58114: [0.18906, 0.69141, 0, 0, 0.33328],
    58115: [0.18906, 0.69141, 0, 0, 0.32923],
    58116: [0.18906, 0.47534, 0, 0, 0.50343],
    58117: [0, 0.69141, 0, 0, 0.33301],
    58118: [0, 0.62119, 0, 0, 0.33409],
    58119: [0, 0.47534, 0, 0, 0.50073]
  },
  "Main-Bold": {
    32: [0, 0, 0, 0, 0.25],
    33: [0, 0.69444, 0, 0, 0.35],
    34: [0, 0.69444, 0, 0, 0.60278],
    35: [0.19444, 0.69444, 0, 0, 0.95833],
    36: [0.05556, 0.75, 0, 0, 0.575],
    37: [0.05556, 0.75, 0, 0, 0.95833],
    38: [0, 0.69444, 0, 0, 0.89444],
    39: [0, 0.69444, 0, 0, 0.31944],
    40: [0.25, 0.75, 0, 0, 0.44722],
    41: [0.25, 0.75, 0, 0, 0.44722],
    42: [0, 0.75, 0, 0, 0.575],
    43: [0.13333, 0.63333, 0, 0, 0.89444],
    44: [0.19444, 0.15556, 0, 0, 0.31944],
    45: [0, 0.44444, 0, 0, 0.38333],
    46: [0, 0.15556, 0, 0, 0.31944],
    47: [0.25, 0.75, 0, 0, 0.575],
    48: [0, 0.64444, 0, 0, 0.575],
    49: [0, 0.64444, 0, 0, 0.575],
    50: [0, 0.64444, 0, 0, 0.575],
    51: [0, 0.64444, 0, 0, 0.575],
    52: [0, 0.64444, 0, 0, 0.575],
    53: [0, 0.64444, 0, 0, 0.575],
    54: [0, 0.64444, 0, 0, 0.575],
    55: [0, 0.64444, 0, 0, 0.575],
    56: [0, 0.64444, 0, 0, 0.575],
    57: [0, 0.64444, 0, 0, 0.575],
    58: [0, 0.44444, 0, 0, 0.31944],
    59: [0.19444, 0.44444, 0, 0, 0.31944],
    60: [0.08556, 0.58556, 0, 0, 0.89444],
    61: [-0.10889, 0.39111, 0, 0, 0.89444],
    62: [0.08556, 0.58556, 0, 0, 0.89444],
    63: [0, 0.69444, 0, 0, 0.54305],
    64: [0, 0.69444, 0, 0, 0.89444],
    65: [0, 0.68611, 0, 0, 0.86944],
    66: [0, 0.68611, 0, 0, 0.81805],
    67: [0, 0.68611, 0, 0, 0.83055],
    68: [0, 0.68611, 0, 0, 0.88194],
    69: [0, 0.68611, 0, 0, 0.75555],
    70: [0, 0.68611, 0, 0, 0.72361],
    71: [0, 0.68611, 0, 0, 0.90416],
    72: [0, 0.68611, 0, 0, 0.9],
    73: [0, 0.68611, 0, 0, 0.43611],
    74: [0, 0.68611, 0, 0, 0.59444],
    75: [0, 0.68611, 0, 0, 0.90138],
    76: [0, 0.68611, 0, 0, 0.69166],
    77: [0, 0.68611, 0, 0, 1.09166],
    78: [0, 0.68611, 0, 0, 0.9],
    79: [0, 0.68611, 0, 0, 0.86388],
    80: [0, 0.68611, 0, 0, 0.78611],
    81: [0.19444, 0.68611, 0, 0, 0.86388],
    82: [0, 0.68611, 0, 0, 0.8625],
    83: [0, 0.68611, 0, 0, 0.63889],
    84: [0, 0.68611, 0, 0, 0.8],
    85: [0, 0.68611, 0, 0, 0.88472],
    86: [0, 0.68611, 0.01597, 0, 0.86944],
    87: [0, 0.68611, 0.01597, 0, 1.18888],
    88: [0, 0.68611, 0, 0, 0.86944],
    89: [0, 0.68611, 0.02875, 0, 0.86944],
    90: [0, 0.68611, 0, 0, 0.70277],
    91: [0.25, 0.75, 0, 0, 0.31944],
    92: [0.25, 0.75, 0, 0, 0.575],
    93: [0.25, 0.75, 0, 0, 0.31944],
    94: [0, 0.69444, 0, 0, 0.575],
    95: [0.31, 0.13444, 0.03194, 0, 0.575],
    97: [0, 0.44444, 0, 0, 0.55902],
    98: [0, 0.69444, 0, 0, 0.63889],
    99: [0, 0.44444, 0, 0, 0.51111],
    100: [0, 0.69444, 0, 0, 0.63889],
    101: [0, 0.44444, 0, 0, 0.52708],
    102: [0, 0.69444, 0.10903, 0, 0.35139],
    103: [0.19444, 0.44444, 0.01597, 0, 0.575],
    104: [0, 0.69444, 0, 0, 0.63889],
    105: [0, 0.69444, 0, 0, 0.31944],
    106: [0.19444, 0.69444, 0, 0, 0.35139],
    107: [0, 0.69444, 0, 0, 0.60694],
    108: [0, 0.69444, 0, 0, 0.31944],
    109: [0, 0.44444, 0, 0, 0.95833],
    110: [0, 0.44444, 0, 0, 0.63889],
    111: [0, 0.44444, 0, 0, 0.575],
    112: [0.19444, 0.44444, 0, 0, 0.63889],
    113: [0.19444, 0.44444, 0, 0, 0.60694],
    114: [0, 0.44444, 0, 0, 0.47361],
    115: [0, 0.44444, 0, 0, 0.45361],
    116: [0, 0.63492, 0, 0, 0.44722],
    117: [0, 0.44444, 0, 0, 0.63889],
    118: [0, 0.44444, 0.01597, 0, 0.60694],
    119: [0, 0.44444, 0.01597, 0, 0.83055],
    120: [0, 0.44444, 0, 0, 0.60694],
    121: [0.19444, 0.44444, 0.01597, 0, 0.60694],
    122: [0, 0.44444, 0, 0, 0.51111],
    123: [0.25, 0.75, 0, 0, 0.575],
    124: [0.25, 0.75, 0, 0, 0.31944],
    125: [0.25, 0.75, 0, 0, 0.575],
    126: [0.35, 0.34444, 0, 0, 0.575],
    160: [0, 0, 0, 0, 0.25],
    163: [0, 0.69444, 0, 0, 0.86853],
    168: [0, 0.69444, 0, 0, 0.575],
    172: [0, 0.44444, 0, 0, 0.76666],
    176: [0, 0.69444, 0, 0, 0.86944],
    177: [0.13333, 0.63333, 0, 0, 0.89444],
    184: [0.17014, 0, 0, 0, 0.51111],
    198: [0, 0.68611, 0, 0, 1.04166],
    215: [0.13333, 0.63333, 0, 0, 0.89444],
    216: [0.04861, 0.73472, 0, 0, 0.89444],
    223: [0, 0.69444, 0, 0, 0.59722],
    230: [0, 0.44444, 0, 0, 0.83055],
    247: [0.13333, 0.63333, 0, 0, 0.89444],
    248: [0.09722, 0.54167, 0, 0, 0.575],
    305: [0, 0.44444, 0, 0, 0.31944],
    338: [0, 0.68611, 0, 0, 1.16944],
    339: [0, 0.44444, 0, 0, 0.89444],
    567: [0.19444, 0.44444, 0, 0, 0.35139],
    710: [0, 0.69444, 0, 0, 0.575],
    711: [0, 0.63194, 0, 0, 0.575],
    713: [0, 0.59611, 0, 0, 0.575],
    714: [0, 0.69444, 0, 0, 0.575],
    715: [0, 0.69444, 0, 0, 0.575],
    728: [0, 0.69444, 0, 0, 0.575],
    729: [0, 0.69444, 0, 0, 0.31944],
    730: [0, 0.69444, 0, 0, 0.86944],
    732: [0, 0.69444, 0, 0, 0.575],
    733: [0, 0.69444, 0, 0, 0.575],
    915: [0, 0.68611, 0, 0, 0.69166],
    916: [0, 0.68611, 0, 0, 0.95833],
    920: [0, 0.68611, 0, 0, 0.89444],
    923: [0, 0.68611, 0, 0, 0.80555],
    926: [0, 0.68611, 0, 0, 0.76666],
    928: [0, 0.68611, 0, 0, 0.9],
    931: [0, 0.68611, 0, 0, 0.83055],
    933: [0, 0.68611, 0, 0, 0.89444],
    934: [0, 0.68611, 0, 0, 0.83055],
    936: [0, 0.68611, 0, 0, 0.89444],
    937: [0, 0.68611, 0, 0, 0.83055],
    8211: [0, 0.44444, 0.03194, 0, 0.575],
    8212: [0, 0.44444, 0.03194, 0, 1.14999],
    8216: [0, 0.69444, 0, 0, 0.31944],
    8217: [0, 0.69444, 0, 0, 0.31944],
    8220: [0, 0.69444, 0, 0, 0.60278],
    8221: [0, 0.69444, 0, 0, 0.60278],
    8224: [0.19444, 0.69444, 0, 0, 0.51111],
    8225: [0.19444, 0.69444, 0, 0, 0.51111],
    8242: [0, 0.55556, 0, 0, 0.34444],
    8407: [0, 0.72444, 0.15486, 0, 0.575],
    8463: [0, 0.69444, 0, 0, 0.66759],
    8465: [0, 0.69444, 0, 0, 0.83055],
    8467: [0, 0.69444, 0, 0, 0.47361],
    8472: [0.19444, 0.44444, 0, 0, 0.74027],
    8476: [0, 0.69444, 0, 0, 0.83055],
    8501: [0, 0.69444, 0, 0, 0.70277],
    8592: [-0.10889, 0.39111, 0, 0, 1.14999],
    8593: [0.19444, 0.69444, 0, 0, 0.575],
    8594: [-0.10889, 0.39111, 0, 0, 1.14999],
    8595: [0.19444, 0.69444, 0, 0, 0.575],
    8596: [-0.10889, 0.39111, 0, 0, 1.14999],
    8597: [0.25, 0.75, 0, 0, 0.575],
    8598: [0.19444, 0.69444, 0, 0, 1.14999],
    8599: [0.19444, 0.69444, 0, 0, 1.14999],
    8600: [0.19444, 0.69444, 0, 0, 1.14999],
    8601: [0.19444, 0.69444, 0, 0, 1.14999],
    8636: [-0.10889, 0.39111, 0, 0, 1.14999],
    8637: [-0.10889, 0.39111, 0, 0, 1.14999],
    8640: [-0.10889, 0.39111, 0, 0, 1.14999],
    8641: [-0.10889, 0.39111, 0, 0, 1.14999],
    8656: [-0.10889, 0.39111, 0, 0, 1.14999],
    8657: [0.19444, 0.69444, 0, 0, 0.70277],
    8658: [-0.10889, 0.39111, 0, 0, 1.14999],
    8659: [0.19444, 0.69444, 0, 0, 0.70277],
    8660: [-0.10889, 0.39111, 0, 0, 1.14999],
    8661: [0.25, 0.75, 0, 0, 0.70277],
    8704: [0, 0.69444, 0, 0, 0.63889],
    8706: [0, 0.69444, 0.06389, 0, 0.62847],
    8707: [0, 0.69444, 0, 0, 0.63889],
    8709: [0.05556, 0.75, 0, 0, 0.575],
    8711: [0, 0.68611, 0, 0, 0.95833],
    8712: [0.08556, 0.58556, 0, 0, 0.76666],
    8715: [0.08556, 0.58556, 0, 0, 0.76666],
    8722: [0.13333, 0.63333, 0, 0, 0.89444],
    8723: [0.13333, 0.63333, 0, 0, 0.89444],
    8725: [0.25, 0.75, 0, 0, 0.575],
    8726: [0.25, 0.75, 0, 0, 0.575],
    8727: [-0.02778, 0.47222, 0, 0, 0.575],
    8728: [-0.02639, 0.47361, 0, 0, 0.575],
    8729: [-0.02639, 0.47361, 0, 0, 0.575],
    8730: [0.18, 0.82, 0, 0, 0.95833],
    8733: [0, 0.44444, 0, 0, 0.89444],
    8734: [0, 0.44444, 0, 0, 1.14999],
    8736: [0, 0.69224, 0, 0, 0.72222],
    8739: [0.25, 0.75, 0, 0, 0.31944],
    8741: [0.25, 0.75, 0, 0, 0.575],
    8743: [0, 0.55556, 0, 0, 0.76666],
    8744: [0, 0.55556, 0, 0, 0.76666],
    8745: [0, 0.55556, 0, 0, 0.76666],
    8746: [0, 0.55556, 0, 0, 0.76666],
    8747: [0.19444, 0.69444, 0.12778, 0, 0.56875],
    8764: [-0.10889, 0.39111, 0, 0, 0.89444],
    8768: [0.19444, 0.69444, 0, 0, 0.31944],
    8771: [222e-5, 0.50222, 0, 0, 0.89444],
    8773: [0.027, 0.638, 0, 0, 0.894],
    8776: [0.02444, 0.52444, 0, 0, 0.89444],
    8781: [222e-5, 0.50222, 0, 0, 0.89444],
    8801: [222e-5, 0.50222, 0, 0, 0.89444],
    8804: [0.19667, 0.69667, 0, 0, 0.89444],
    8805: [0.19667, 0.69667, 0, 0, 0.89444],
    8810: [0.08556, 0.58556, 0, 0, 1.14999],
    8811: [0.08556, 0.58556, 0, 0, 1.14999],
    8826: [0.08556, 0.58556, 0, 0, 0.89444],
    8827: [0.08556, 0.58556, 0, 0, 0.89444],
    8834: [0.08556, 0.58556, 0, 0, 0.89444],
    8835: [0.08556, 0.58556, 0, 0, 0.89444],
    8838: [0.19667, 0.69667, 0, 0, 0.89444],
    8839: [0.19667, 0.69667, 0, 0, 0.89444],
    8846: [0, 0.55556, 0, 0, 0.76666],
    8849: [0.19667, 0.69667, 0, 0, 0.89444],
    8850: [0.19667, 0.69667, 0, 0, 0.89444],
    8851: [0, 0.55556, 0, 0, 0.76666],
    8852: [0, 0.55556, 0, 0, 0.76666],
    8853: [0.13333, 0.63333, 0, 0, 0.89444],
    8854: [0.13333, 0.63333, 0, 0, 0.89444],
    8855: [0.13333, 0.63333, 0, 0, 0.89444],
    8856: [0.13333, 0.63333, 0, 0, 0.89444],
    8857: [0.13333, 0.63333, 0, 0, 0.89444],
    8866: [0, 0.69444, 0, 0, 0.70277],
    8867: [0, 0.69444, 0, 0, 0.70277],
    8868: [0, 0.69444, 0, 0, 0.89444],
    8869: [0, 0.69444, 0, 0, 0.89444],
    8900: [-0.02639, 0.47361, 0, 0, 0.575],
    8901: [-0.02639, 0.47361, 0, 0, 0.31944],
    8902: [-0.02778, 0.47222, 0, 0, 0.575],
    8968: [0.25, 0.75, 0, 0, 0.51111],
    8969: [0.25, 0.75, 0, 0, 0.51111],
    8970: [0.25, 0.75, 0, 0, 0.51111],
    8971: [0.25, 0.75, 0, 0, 0.51111],
    8994: [-0.13889, 0.36111, 0, 0, 1.14999],
    8995: [-0.13889, 0.36111, 0, 0, 1.14999],
    9651: [0.19444, 0.69444, 0, 0, 1.02222],
    9657: [-0.02778, 0.47222, 0, 0, 0.575],
    9661: [0.19444, 0.69444, 0, 0, 1.02222],
    9667: [-0.02778, 0.47222, 0, 0, 0.575],
    9711: [0.19444, 0.69444, 0, 0, 1.14999],
    9824: [0.12963, 0.69444, 0, 0, 0.89444],
    9825: [0.12963, 0.69444, 0, 0, 0.89444],
    9826: [0.12963, 0.69444, 0, 0, 0.89444],
    9827: [0.12963, 0.69444, 0, 0, 0.89444],
    9837: [0, 0.75, 0, 0, 0.44722],
    9838: [0.19444, 0.69444, 0, 0, 0.44722],
    9839: [0.19444, 0.69444, 0, 0, 0.44722],
    10216: [0.25, 0.75, 0, 0, 0.44722],
    10217: [0.25, 0.75, 0, 0, 0.44722],
    10815: [0, 0.68611, 0, 0, 0.9],
    10927: [0.19667, 0.69667, 0, 0, 0.89444],
    10928: [0.19667, 0.69667, 0, 0, 0.89444],
    57376: [0.19444, 0.69444, 0, 0, 0]
  },
  "Main-BoldItalic": {
    32: [0, 0, 0, 0, 0.25],
    33: [0, 0.69444, 0.11417, 0, 0.38611],
    34: [0, 0.69444, 0.07939, 0, 0.62055],
    35: [0.19444, 0.69444, 0.06833, 0, 0.94444],
    37: [0.05556, 0.75, 0.12861, 0, 0.94444],
    38: [0, 0.69444, 0.08528, 0, 0.88555],
    39: [0, 0.69444, 0.12945, 0, 0.35555],
    40: [0.25, 0.75, 0.15806, 0, 0.47333],
    41: [0.25, 0.75, 0.03306, 0, 0.47333],
    42: [0, 0.75, 0.14333, 0, 0.59111],
    43: [0.10333, 0.60333, 0.03306, 0, 0.88555],
    44: [0.19444, 0.14722, 0, 0, 0.35555],
    45: [0, 0.44444, 0.02611, 0, 0.41444],
    46: [0, 0.14722, 0, 0, 0.35555],
    47: [0.25, 0.75, 0.15806, 0, 0.59111],
    48: [0, 0.64444, 0.13167, 0, 0.59111],
    49: [0, 0.64444, 0.13167, 0, 0.59111],
    50: [0, 0.64444, 0.13167, 0, 0.59111],
    51: [0, 0.64444, 0.13167, 0, 0.59111],
    52: [0.19444, 0.64444, 0.13167, 0, 0.59111],
    53: [0, 0.64444, 0.13167, 0, 0.59111],
    54: [0, 0.64444, 0.13167, 0, 0.59111],
    55: [0.19444, 0.64444, 0.13167, 0, 0.59111],
    56: [0, 0.64444, 0.13167, 0, 0.59111],
    57: [0, 0.64444, 0.13167, 0, 0.59111],
    58: [0, 0.44444, 0.06695, 0, 0.35555],
    59: [0.19444, 0.44444, 0.06695, 0, 0.35555],
    61: [-0.10889, 0.39111, 0.06833, 0, 0.88555],
    63: [0, 0.69444, 0.11472, 0, 0.59111],
    64: [0, 0.69444, 0.09208, 0, 0.88555],
    65: [0, 0.68611, 0, 0, 0.86555],
    66: [0, 0.68611, 0.0992, 0, 0.81666],
    67: [0, 0.68611, 0.14208, 0, 0.82666],
    68: [0, 0.68611, 0.09062, 0, 0.87555],
    69: [0, 0.68611, 0.11431, 0, 0.75666],
    70: [0, 0.68611, 0.12903, 0, 0.72722],
    71: [0, 0.68611, 0.07347, 0, 0.89527],
    72: [0, 0.68611, 0.17208, 0, 0.8961],
    73: [0, 0.68611, 0.15681, 0, 0.47166],
    74: [0, 0.68611, 0.145, 0, 0.61055],
    75: [0, 0.68611, 0.14208, 0, 0.89499],
    76: [0, 0.68611, 0, 0, 0.69777],
    77: [0, 0.68611, 0.17208, 0, 1.07277],
    78: [0, 0.68611, 0.17208, 0, 0.8961],
    79: [0, 0.68611, 0.09062, 0, 0.85499],
    80: [0, 0.68611, 0.0992, 0, 0.78721],
    81: [0.19444, 0.68611, 0.09062, 0, 0.85499],
    82: [0, 0.68611, 0.02559, 0, 0.85944],
    83: [0, 0.68611, 0.11264, 0, 0.64999],
    84: [0, 0.68611, 0.12903, 0, 0.7961],
    85: [0, 0.68611, 0.17208, 0, 0.88083],
    86: [0, 0.68611, 0.18625, 0, 0.86555],
    87: [0, 0.68611, 0.18625, 0, 1.15999],
    88: [0, 0.68611, 0.15681, 0, 0.86555],
    89: [0, 0.68611, 0.19803, 0, 0.86555],
    90: [0, 0.68611, 0.14208, 0, 0.70888],
    91: [0.25, 0.75, 0.1875, 0, 0.35611],
    93: [0.25, 0.75, 0.09972, 0, 0.35611],
    94: [0, 0.69444, 0.06709, 0, 0.59111],
    95: [0.31, 0.13444, 0.09811, 0, 0.59111],
    97: [0, 0.44444, 0.09426, 0, 0.59111],
    98: [0, 0.69444, 0.07861, 0, 0.53222],
    99: [0, 0.44444, 0.05222, 0, 0.53222],
    100: [0, 0.69444, 0.10861, 0, 0.59111],
    101: [0, 0.44444, 0.085, 0, 0.53222],
    102: [0.19444, 0.69444, 0.21778, 0, 0.4],
    103: [0.19444, 0.44444, 0.105, 0, 0.53222],
    104: [0, 0.69444, 0.09426, 0, 0.59111],
    105: [0, 0.69326, 0.11387, 0, 0.35555],
    106: [0.19444, 0.69326, 0.1672, 0, 0.35555],
    107: [0, 0.69444, 0.11111, 0, 0.53222],
    108: [0, 0.69444, 0.10861, 0, 0.29666],
    109: [0, 0.44444, 0.09426, 0, 0.94444],
    110: [0, 0.44444, 0.09426, 0, 0.64999],
    111: [0, 0.44444, 0.07861, 0, 0.59111],
    112: [0.19444, 0.44444, 0.07861, 0, 0.59111],
    113: [0.19444, 0.44444, 0.105, 0, 0.53222],
    114: [0, 0.44444, 0.11111, 0, 0.50167],
    115: [0, 0.44444, 0.08167, 0, 0.48694],
    116: [0, 0.63492, 0.09639, 0, 0.385],
    117: [0, 0.44444, 0.09426, 0, 0.62055],
    118: [0, 0.44444, 0.11111, 0, 0.53222],
    119: [0, 0.44444, 0.11111, 0, 0.76777],
    120: [0, 0.44444, 0.12583, 0, 0.56055],
    121: [0.19444, 0.44444, 0.105, 0, 0.56166],
    122: [0, 0.44444, 0.13889, 0, 0.49055],
    126: [0.35, 0.34444, 0.11472, 0, 0.59111],
    160: [0, 0, 0, 0, 0.25],
    168: [0, 0.69444, 0.11473, 0, 0.59111],
    176: [0, 0.69444, 0, 0, 0.94888],
    184: [0.17014, 0, 0, 0, 0.53222],
    198: [0, 0.68611, 0.11431, 0, 1.02277],
    216: [0.04861, 0.73472, 0.09062, 0, 0.88555],
    223: [0.19444, 0.69444, 0.09736, 0, 0.665],
    230: [0, 0.44444, 0.085, 0, 0.82666],
    248: [0.09722, 0.54167, 0.09458, 0, 0.59111],
    305: [0, 0.44444, 0.09426, 0, 0.35555],
    338: [0, 0.68611, 0.11431, 0, 1.14054],
    339: [0, 0.44444, 0.085, 0, 0.82666],
    567: [0.19444, 0.44444, 0.04611, 0, 0.385],
    710: [0, 0.69444, 0.06709, 0, 0.59111],
    711: [0, 0.63194, 0.08271, 0, 0.59111],
    713: [0, 0.59444, 0.10444, 0, 0.59111],
    714: [0, 0.69444, 0.08528, 0, 0.59111],
    715: [0, 0.69444, 0, 0, 0.59111],
    728: [0, 0.69444, 0.10333, 0, 0.59111],
    729: [0, 0.69444, 0.12945, 0, 0.35555],
    730: [0, 0.69444, 0, 0, 0.94888],
    732: [0, 0.69444, 0.11472, 0, 0.59111],
    733: [0, 0.69444, 0.11472, 0, 0.59111],
    915: [0, 0.68611, 0.12903, 0, 0.69777],
    916: [0, 0.68611, 0, 0, 0.94444],
    920: [0, 0.68611, 0.09062, 0, 0.88555],
    923: [0, 0.68611, 0, 0, 0.80666],
    926: [0, 0.68611, 0.15092, 0, 0.76777],
    928: [0, 0.68611, 0.17208, 0, 0.8961],
    931: [0, 0.68611, 0.11431, 0, 0.82666],
    933: [0, 0.68611, 0.10778, 0, 0.88555],
    934: [0, 0.68611, 0.05632, 0, 0.82666],
    936: [0, 0.68611, 0.10778, 0, 0.88555],
    937: [0, 0.68611, 0.0992, 0, 0.82666],
    8211: [0, 0.44444, 0.09811, 0, 0.59111],
    8212: [0, 0.44444, 0.09811, 0, 1.18221],
    8216: [0, 0.69444, 0.12945, 0, 0.35555],
    8217: [0, 0.69444, 0.12945, 0, 0.35555],
    8220: [0, 0.69444, 0.16772, 0, 0.62055],
    8221: [0, 0.69444, 0.07939, 0, 0.62055]
  },
  "Main-Italic": {
    32: [0, 0, 0, 0, 0.25],
    33: [0, 0.69444, 0.12417, 0, 0.30667],
    34: [0, 0.69444, 0.06961, 0, 0.51444],
    35: [0.19444, 0.69444, 0.06616, 0, 0.81777],
    37: [0.05556, 0.75, 0.13639, 0, 0.81777],
    38: [0, 0.69444, 0.09694, 0, 0.76666],
    39: [0, 0.69444, 0.12417, 0, 0.30667],
    40: [0.25, 0.75, 0.16194, 0, 0.40889],
    41: [0.25, 0.75, 0.03694, 0, 0.40889],
    42: [0, 0.75, 0.14917, 0, 0.51111],
    43: [0.05667, 0.56167, 0.03694, 0, 0.76666],
    44: [0.19444, 0.10556, 0, 0, 0.30667],
    45: [0, 0.43056, 0.02826, 0, 0.35778],
    46: [0, 0.10556, 0, 0, 0.30667],
    47: [0.25, 0.75, 0.16194, 0, 0.51111],
    48: [0, 0.64444, 0.13556, 0, 0.51111],
    49: [0, 0.64444, 0.13556, 0, 0.51111],
    50: [0, 0.64444, 0.13556, 0, 0.51111],
    51: [0, 0.64444, 0.13556, 0, 0.51111],
    52: [0.19444, 0.64444, 0.13556, 0, 0.51111],
    53: [0, 0.64444, 0.13556, 0, 0.51111],
    54: [0, 0.64444, 0.13556, 0, 0.51111],
    55: [0.19444, 0.64444, 0.13556, 0, 0.51111],
    56: [0, 0.64444, 0.13556, 0, 0.51111],
    57: [0, 0.64444, 0.13556, 0, 0.51111],
    58: [0, 0.43056, 0.0582, 0, 0.30667],
    59: [0.19444, 0.43056, 0.0582, 0, 0.30667],
    61: [-0.13313, 0.36687, 0.06616, 0, 0.76666],
    63: [0, 0.69444, 0.1225, 0, 0.51111],
    64: [0, 0.69444, 0.09597, 0, 0.76666],
    65: [0, 0.68333, 0, 0, 0.74333],
    66: [0, 0.68333, 0.10257, 0, 0.70389],
    67: [0, 0.68333, 0.14528, 0, 0.71555],
    68: [0, 0.68333, 0.09403, 0, 0.755],
    69: [0, 0.68333, 0.12028, 0, 0.67833],
    70: [0, 0.68333, 0.13305, 0, 0.65277],
    71: [0, 0.68333, 0.08722, 0, 0.77361],
    72: [0, 0.68333, 0.16389, 0, 0.74333],
    73: [0, 0.68333, 0.15806, 0, 0.38555],
    74: [0, 0.68333, 0.14028, 0, 0.525],
    75: [0, 0.68333, 0.14528, 0, 0.76888],
    76: [0, 0.68333, 0, 0, 0.62722],
    77: [0, 0.68333, 0.16389, 0, 0.89666],
    78: [0, 0.68333, 0.16389, 0, 0.74333],
    79: [0, 0.68333, 0.09403, 0, 0.76666],
    80: [0, 0.68333, 0.10257, 0, 0.67833],
    81: [0.19444, 0.68333, 0.09403, 0, 0.76666],
    82: [0, 0.68333, 0.03868, 0, 0.72944],
    83: [0, 0.68333, 0.11972, 0, 0.56222],
    84: [0, 0.68333, 0.13305, 0, 0.71555],
    85: [0, 0.68333, 0.16389, 0, 0.74333],
    86: [0, 0.68333, 0.18361, 0, 0.74333],
    87: [0, 0.68333, 0.18361, 0, 0.99888],
    88: [0, 0.68333, 0.15806, 0, 0.74333],
    89: [0, 0.68333, 0.19383, 0, 0.74333],
    90: [0, 0.68333, 0.14528, 0, 0.61333],
    91: [0.25, 0.75, 0.1875, 0, 0.30667],
    93: [0.25, 0.75, 0.10528, 0, 0.30667],
    94: [0, 0.69444, 0.06646, 0, 0.51111],
    95: [0.31, 0.12056, 0.09208, 0, 0.51111],
    97: [0, 0.43056, 0.07671, 0, 0.51111],
    98: [0, 0.69444, 0.06312, 0, 0.46],
    99: [0, 0.43056, 0.05653, 0, 0.46],
    100: [0, 0.69444, 0.10333, 0, 0.51111],
    101: [0, 0.43056, 0.07514, 0, 0.46],
    102: [0.19444, 0.69444, 0.21194, 0, 0.30667],
    103: [0.19444, 0.43056, 0.08847, 0, 0.46],
    104: [0, 0.69444, 0.07671, 0, 0.51111],
    105: [0, 0.65536, 0.1019, 0, 0.30667],
    106: [0.19444, 0.65536, 0.14467, 0, 0.30667],
    107: [0, 0.69444, 0.10764, 0, 0.46],
    108: [0, 0.69444, 0.10333, 0, 0.25555],
    109: [0, 0.43056, 0.07671, 0, 0.81777],
    110: [0, 0.43056, 0.07671, 0, 0.56222],
    111: [0, 0.43056, 0.06312, 0, 0.51111],
    112: [0.19444, 0.43056, 0.06312, 0, 0.51111],
    113: [0.19444, 0.43056, 0.08847, 0, 0.46],
    114: [0, 0.43056, 0.10764, 0, 0.42166],
    115: [0, 0.43056, 0.08208, 0, 0.40889],
    116: [0, 0.61508, 0.09486, 0, 0.33222],
    117: [0, 0.43056, 0.07671, 0, 0.53666],
    118: [0, 0.43056, 0.10764, 0, 0.46],
    119: [0, 0.43056, 0.10764, 0, 0.66444],
    120: [0, 0.43056, 0.12042, 0, 0.46389],
    121: [0.19444, 0.43056, 0.08847, 0, 0.48555],
    122: [0, 0.43056, 0.12292, 0, 0.40889],
    126: [0.35, 0.31786, 0.11585, 0, 0.51111],
    160: [0, 0, 0, 0, 0.25],
    168: [0, 0.66786, 0.10474, 0, 0.51111],
    176: [0, 0.69444, 0, 0, 0.83129],
    184: [0.17014, 0, 0, 0, 0.46],
    198: [0, 0.68333, 0.12028, 0, 0.88277],
    216: [0.04861, 0.73194, 0.09403, 0, 0.76666],
    223: [0.19444, 0.69444, 0.10514, 0, 0.53666],
    230: [0, 0.43056, 0.07514, 0, 0.71555],
    248: [0.09722, 0.52778, 0.09194, 0, 0.51111],
    338: [0, 0.68333, 0.12028, 0, 0.98499],
    339: [0, 0.43056, 0.07514, 0, 0.71555],
    710: [0, 0.69444, 0.06646, 0, 0.51111],
    711: [0, 0.62847, 0.08295, 0, 0.51111],
    713: [0, 0.56167, 0.10333, 0, 0.51111],
    714: [0, 0.69444, 0.09694, 0, 0.51111],
    715: [0, 0.69444, 0, 0, 0.51111],
    728: [0, 0.69444, 0.10806, 0, 0.51111],
    729: [0, 0.66786, 0.11752, 0, 0.30667],
    730: [0, 0.69444, 0, 0, 0.83129],
    732: [0, 0.66786, 0.11585, 0, 0.51111],
    733: [0, 0.69444, 0.1225, 0, 0.51111],
    915: [0, 0.68333, 0.13305, 0, 0.62722],
    916: [0, 0.68333, 0, 0, 0.81777],
    920: [0, 0.68333, 0.09403, 0, 0.76666],
    923: [0, 0.68333, 0, 0, 0.69222],
    926: [0, 0.68333, 0.15294, 0, 0.66444],
    928: [0, 0.68333, 0.16389, 0, 0.74333],
    931: [0, 0.68333, 0.12028, 0, 0.71555],
    933: [0, 0.68333, 0.11111, 0, 0.76666],
    934: [0, 0.68333, 0.05986, 0, 0.71555],
    936: [0, 0.68333, 0.11111, 0, 0.76666],
    937: [0, 0.68333, 0.10257, 0, 0.71555],
    8211: [0, 0.43056, 0.09208, 0, 0.51111],
    8212: [0, 0.43056, 0.09208, 0, 1.02222],
    8216: [0, 0.69444, 0.12417, 0, 0.30667],
    8217: [0, 0.69444, 0.12417, 0, 0.30667],
    8220: [0, 0.69444, 0.1685, 0, 0.51444],
    8221: [0, 0.69444, 0.06961, 0, 0.51444],
    8463: [0, 0.68889, 0, 0, 0.54028]
  },
  "Main-Regular": {
    32: [0, 0, 0, 0, 0.25],
    33: [0, 0.69444, 0, 0, 0.27778],
    34: [0, 0.69444, 0, 0, 0.5],
    35: [0.19444, 0.69444, 0, 0, 0.83334],
    36: [0.05556, 0.75, 0, 0, 0.5],
    37: [0.05556, 0.75, 0, 0, 0.83334],
    38: [0, 0.69444, 0, 0, 0.77778],
    39: [0, 0.69444, 0, 0, 0.27778],
    40: [0.25, 0.75, 0, 0, 0.38889],
    41: [0.25, 0.75, 0, 0, 0.38889],
    42: [0, 0.75, 0, 0, 0.5],
    43: [0.08333, 0.58333, 0, 0, 0.77778],
    44: [0.19444, 0.10556, 0, 0, 0.27778],
    45: [0, 0.43056, 0, 0, 0.33333],
    46: [0, 0.10556, 0, 0, 0.27778],
    47: [0.25, 0.75, 0, 0, 0.5],
    48: [0, 0.64444, 0, 0, 0.5],
    49: [0, 0.64444, 0, 0, 0.5],
    50: [0, 0.64444, 0, 0, 0.5],
    51: [0, 0.64444, 0, 0, 0.5],
    52: [0, 0.64444, 0, 0, 0.5],
    53: [0, 0.64444, 0, 0, 0.5],
    54: [0, 0.64444, 0, 0, 0.5],
    55: [0, 0.64444, 0, 0, 0.5],
    56: [0, 0.64444, 0, 0, 0.5],
    57: [0, 0.64444, 0, 0, 0.5],
    58: [0, 0.43056, 0, 0, 0.27778],
    59: [0.19444, 0.43056, 0, 0, 0.27778],
    60: [0.0391, 0.5391, 0, 0, 0.77778],
    61: [-0.13313, 0.36687, 0, 0, 0.77778],
    62: [0.0391, 0.5391, 0, 0, 0.77778],
    63: [0, 0.69444, 0, 0, 0.47222],
    64: [0, 0.69444, 0, 0, 0.77778],
    65: [0, 0.68333, 0, 0, 0.75],
    66: [0, 0.68333, 0, 0, 0.70834],
    67: [0, 0.68333, 0, 0, 0.72222],
    68: [0, 0.68333, 0, 0, 0.76389],
    69: [0, 0.68333, 0, 0, 0.68056],
    70: [0, 0.68333, 0, 0, 0.65278],
    71: [0, 0.68333, 0, 0, 0.78472],
    72: [0, 0.68333, 0, 0, 0.75],
    73: [0, 0.68333, 0, 0, 0.36111],
    74: [0, 0.68333, 0, 0, 0.51389],
    75: [0, 0.68333, 0, 0, 0.77778],
    76: [0, 0.68333, 0, 0, 0.625],
    77: [0, 0.68333, 0, 0, 0.91667],
    78: [0, 0.68333, 0, 0, 0.75],
    79: [0, 0.68333, 0, 0, 0.77778],
    80: [0, 0.68333, 0, 0, 0.68056],
    81: [0.19444, 0.68333, 0, 0, 0.77778],
    82: [0, 0.68333, 0, 0, 0.73611],
    83: [0, 0.68333, 0, 0, 0.55556],
    84: [0, 0.68333, 0, 0, 0.72222],
    85: [0, 0.68333, 0, 0, 0.75],
    86: [0, 0.68333, 0.01389, 0, 0.75],
    87: [0, 0.68333, 0.01389, 0, 1.02778],
    88: [0, 0.68333, 0, 0, 0.75],
    89: [0, 0.68333, 0.025, 0, 0.75],
    90: [0, 0.68333, 0, 0, 0.61111],
    91: [0.25, 0.75, 0, 0, 0.27778],
    92: [0.25, 0.75, 0, 0, 0.5],
    93: [0.25, 0.75, 0, 0, 0.27778],
    94: [0, 0.69444, 0, 0, 0.5],
    95: [0.31, 0.12056, 0.02778, 0, 0.5],
    97: [0, 0.43056, 0, 0, 0.5],
    98: [0, 0.69444, 0, 0, 0.55556],
    99: [0, 0.43056, 0, 0, 0.44445],
    100: [0, 0.69444, 0, 0, 0.55556],
    101: [0, 0.43056, 0, 0, 0.44445],
    102: [0, 0.69444, 0.07778, 0, 0.30556],
    103: [0.19444, 0.43056, 0.01389, 0, 0.5],
    104: [0, 0.69444, 0, 0, 0.55556],
    105: [0, 0.66786, 0, 0, 0.27778],
    106: [0.19444, 0.66786, 0, 0, 0.30556],
    107: [0, 0.69444, 0, 0, 0.52778],
    108: [0, 0.69444, 0, 0, 0.27778],
    109: [0, 0.43056, 0, 0, 0.83334],
    110: [0, 0.43056, 0, 0, 0.55556],
    111: [0, 0.43056, 0, 0, 0.5],
    112: [0.19444, 0.43056, 0, 0, 0.55556],
    113: [0.19444, 0.43056, 0, 0, 0.52778],
    114: [0, 0.43056, 0, 0, 0.39167],
    115: [0, 0.43056, 0, 0, 0.39445],
    116: [0, 0.61508, 0, 0, 0.38889],
    117: [0, 0.43056, 0, 0, 0.55556],
    118: [0, 0.43056, 0.01389, 0, 0.52778],
    119: [0, 0.43056, 0.01389, 0, 0.72222],
    120: [0, 0.43056, 0, 0, 0.52778],
    121: [0.19444, 0.43056, 0.01389, 0, 0.52778],
    122: [0, 0.43056, 0, 0, 0.44445],
    123: [0.25, 0.75, 0, 0, 0.5],
    124: [0.25, 0.75, 0, 0, 0.27778],
    125: [0.25, 0.75, 0, 0, 0.5],
    126: [0.35, 0.31786, 0, 0, 0.5],
    160: [0, 0, 0, 0, 0.25],
    163: [0, 0.69444, 0, 0, 0.76909],
    167: [0.19444, 0.69444, 0, 0, 0.44445],
    168: [0, 0.66786, 0, 0, 0.5],
    172: [0, 0.43056, 0, 0, 0.66667],
    176: [0, 0.69444, 0, 0, 0.75],
    177: [0.08333, 0.58333, 0, 0, 0.77778],
    182: [0.19444, 0.69444, 0, 0, 0.61111],
    184: [0.17014, 0, 0, 0, 0.44445],
    198: [0, 0.68333, 0, 0, 0.90278],
    215: [0.08333, 0.58333, 0, 0, 0.77778],
    216: [0.04861, 0.73194, 0, 0, 0.77778],
    223: [0, 0.69444, 0, 0, 0.5],
    230: [0, 0.43056, 0, 0, 0.72222],
    247: [0.08333, 0.58333, 0, 0, 0.77778],
    248: [0.09722, 0.52778, 0, 0, 0.5],
    305: [0, 0.43056, 0, 0, 0.27778],
    338: [0, 0.68333, 0, 0, 1.01389],
    339: [0, 0.43056, 0, 0, 0.77778],
    567: [0.19444, 0.43056, 0, 0, 0.30556],
    710: [0, 0.69444, 0, 0, 0.5],
    711: [0, 0.62847, 0, 0, 0.5],
    713: [0, 0.56778, 0, 0, 0.5],
    714: [0, 0.69444, 0, 0, 0.5],
    715: [0, 0.69444, 0, 0, 0.5],
    728: [0, 0.69444, 0, 0, 0.5],
    729: [0, 0.66786, 0, 0, 0.27778],
    730: [0, 0.69444, 0, 0, 0.75],
    732: [0, 0.66786, 0, 0, 0.5],
    733: [0, 0.69444, 0, 0, 0.5],
    915: [0, 0.68333, 0, 0, 0.625],
    916: [0, 0.68333, 0, 0, 0.83334],
    920: [0, 0.68333, 0, 0, 0.77778],
    923: [0, 0.68333, 0, 0, 0.69445],
    926: [0, 0.68333, 0, 0, 0.66667],
    928: [0, 0.68333, 0, 0, 0.75],
    931: [0, 0.68333, 0, 0, 0.72222],
    933: [0, 0.68333, 0, 0, 0.77778],
    934: [0, 0.68333, 0, 0, 0.72222],
    936: [0, 0.68333, 0, 0, 0.77778],
    937: [0, 0.68333, 0, 0, 0.72222],
    8211: [0, 0.43056, 0.02778, 0, 0.5],
    8212: [0, 0.43056, 0.02778, 0, 1],
    8216: [0, 0.69444, 0, 0, 0.27778],
    8217: [0, 0.69444, 0, 0, 0.27778],
    8220: [0, 0.69444, 0, 0, 0.5],
    8221: [0, 0.69444, 0, 0, 0.5],
    8224: [0.19444, 0.69444, 0, 0, 0.44445],
    8225: [0.19444, 0.69444, 0, 0, 0.44445],
    8230: [0, 0.123, 0, 0, 1.172],
    8242: [0, 0.55556, 0, 0, 0.275],
    8407: [0, 0.71444, 0.15382, 0, 0.5],
    8463: [0, 0.68889, 0, 0, 0.54028],
    8465: [0, 0.69444, 0, 0, 0.72222],
    8467: [0, 0.69444, 0, 0.11111, 0.41667],
    8472: [0.19444, 0.43056, 0, 0.11111, 0.63646],
    8476: [0, 0.69444, 0, 0, 0.72222],
    8501: [0, 0.69444, 0, 0, 0.61111],
    8592: [-0.13313, 0.36687, 0, 0, 1],
    8593: [0.19444, 0.69444, 0, 0, 0.5],
    8594: [-0.13313, 0.36687, 0, 0, 1],
    8595: [0.19444, 0.69444, 0, 0, 0.5],
    8596: [-0.13313, 0.36687, 0, 0, 1],
    8597: [0.25, 0.75, 0, 0, 0.5],
    8598: [0.19444, 0.69444, 0, 0, 1],
    8599: [0.19444, 0.69444, 0, 0, 1],
    8600: [0.19444, 0.69444, 0, 0, 1],
    8601: [0.19444, 0.69444, 0, 0, 1],
    8614: [0.011, 0.511, 0, 0, 1],
    8617: [0.011, 0.511, 0, 0, 1.126],
    8618: [0.011, 0.511, 0, 0, 1.126],
    8636: [-0.13313, 0.36687, 0, 0, 1],
    8637: [-0.13313, 0.36687, 0, 0, 1],
    8640: [-0.13313, 0.36687, 0, 0, 1],
    8641: [-0.13313, 0.36687, 0, 0, 1],
    8652: [0.011, 0.671, 0, 0, 1],
    8656: [-0.13313, 0.36687, 0, 0, 1],
    8657: [0.19444, 0.69444, 0, 0, 0.61111],
    8658: [-0.13313, 0.36687, 0, 0, 1],
    8659: [0.19444, 0.69444, 0, 0, 0.61111],
    8660: [-0.13313, 0.36687, 0, 0, 1],
    8661: [0.25, 0.75, 0, 0, 0.61111],
    8704: [0, 0.69444, 0, 0, 0.55556],
    8706: [0, 0.69444, 0.05556, 0.08334, 0.5309],
    8707: [0, 0.69444, 0, 0, 0.55556],
    8709: [0.05556, 0.75, 0, 0, 0.5],
    8711: [0, 0.68333, 0, 0, 0.83334],
    8712: [0.0391, 0.5391, 0, 0, 0.66667],
    8715: [0.0391, 0.5391, 0, 0, 0.66667],
    8722: [0.08333, 0.58333, 0, 0, 0.77778],
    8723: [0.08333, 0.58333, 0, 0, 0.77778],
    8725: [0.25, 0.75, 0, 0, 0.5],
    8726: [0.25, 0.75, 0, 0, 0.5],
    8727: [-0.03472, 0.46528, 0, 0, 0.5],
    8728: [-0.05555, 0.44445, 0, 0, 0.5],
    8729: [-0.05555, 0.44445, 0, 0, 0.5],
    8730: [0.2, 0.8, 0, 0, 0.83334],
    8733: [0, 0.43056, 0, 0, 0.77778],
    8734: [0, 0.43056, 0, 0, 1],
    8736: [0, 0.69224, 0, 0, 0.72222],
    8739: [0.25, 0.75, 0, 0, 0.27778],
    8741: [0.25, 0.75, 0, 0, 0.5],
    8743: [0, 0.55556, 0, 0, 0.66667],
    8744: [0, 0.55556, 0, 0, 0.66667],
    8745: [0, 0.55556, 0, 0, 0.66667],
    8746: [0, 0.55556, 0, 0, 0.66667],
    8747: [0.19444, 0.69444, 0.11111, 0, 0.41667],
    8764: [-0.13313, 0.36687, 0, 0, 0.77778],
    8768: [0.19444, 0.69444, 0, 0, 0.27778],
    8771: [-0.03625, 0.46375, 0, 0, 0.77778],
    8773: [-0.022, 0.589, 0, 0, 0.778],
    8776: [-0.01688, 0.48312, 0, 0, 0.77778],
    8781: [-0.03625, 0.46375, 0, 0, 0.77778],
    8784: [-0.133, 0.673, 0, 0, 0.778],
    8801: [-0.03625, 0.46375, 0, 0, 0.77778],
    8804: [0.13597, 0.63597, 0, 0, 0.77778],
    8805: [0.13597, 0.63597, 0, 0, 0.77778],
    8810: [0.0391, 0.5391, 0, 0, 1],
    8811: [0.0391, 0.5391, 0, 0, 1],
    8826: [0.0391, 0.5391, 0, 0, 0.77778],
    8827: [0.0391, 0.5391, 0, 0, 0.77778],
    8834: [0.0391, 0.5391, 0, 0, 0.77778],
    8835: [0.0391, 0.5391, 0, 0, 0.77778],
    8838: [0.13597, 0.63597, 0, 0, 0.77778],
    8839: [0.13597, 0.63597, 0, 0, 0.77778],
    8846: [0, 0.55556, 0, 0, 0.66667],
    8849: [0.13597, 0.63597, 0, 0, 0.77778],
    8850: [0.13597, 0.63597, 0, 0, 0.77778],
    8851: [0, 0.55556, 0, 0, 0.66667],
    8852: [0, 0.55556, 0, 0, 0.66667],
    8853: [0.08333, 0.58333, 0, 0, 0.77778],
    8854: [0.08333, 0.58333, 0, 0, 0.77778],
    8855: [0.08333, 0.58333, 0, 0, 0.77778],
    8856: [0.08333, 0.58333, 0, 0, 0.77778],
    8857: [0.08333, 0.58333, 0, 0, 0.77778],
    8866: [0, 0.69444, 0, 0, 0.61111],
    8867: [0, 0.69444, 0, 0, 0.61111],
    8868: [0, 0.69444, 0, 0, 0.77778],
    8869: [0, 0.69444, 0, 0, 0.77778],
    8872: [0.249, 0.75, 0, 0, 0.867],
    8900: [-0.05555, 0.44445, 0, 0, 0.5],
    8901: [-0.05555, 0.44445, 0, 0, 0.27778],
    8902: [-0.03472, 0.46528, 0, 0, 0.5],
    8904: [5e-3, 0.505, 0, 0, 0.9],
    8942: [0.03, 0.903, 0, 0, 0.278],
    8943: [-0.19, 0.313, 0, 0, 1.172],
    8945: [-0.1, 0.823, 0, 0, 1.282],
    8968: [0.25, 0.75, 0, 0, 0.44445],
    8969: [0.25, 0.75, 0, 0, 0.44445],
    8970: [0.25, 0.75, 0, 0, 0.44445],
    8971: [0.25, 0.75, 0, 0, 0.44445],
    8994: [-0.14236, 0.35764, 0, 0, 1],
    8995: [-0.14236, 0.35764, 0, 0, 1],
    9136: [0.244, 0.744, 0, 0, 0.412],
    9137: [0.244, 0.745, 0, 0, 0.412],
    9651: [0.19444, 0.69444, 0, 0, 0.88889],
    9657: [-0.03472, 0.46528, 0, 0, 0.5],
    9661: [0.19444, 0.69444, 0, 0, 0.88889],
    9667: [-0.03472, 0.46528, 0, 0, 0.5],
    9711: [0.19444, 0.69444, 0, 0, 1],
    9824: [0.12963, 0.69444, 0, 0, 0.77778],
    9825: [0.12963, 0.69444, 0, 0, 0.77778],
    9826: [0.12963, 0.69444, 0, 0, 0.77778],
    9827: [0.12963, 0.69444, 0, 0, 0.77778],
    9837: [0, 0.75, 0, 0, 0.38889],
    9838: [0.19444, 0.69444, 0, 0, 0.38889],
    9839: [0.19444, 0.69444, 0, 0, 0.38889],
    10216: [0.25, 0.75, 0, 0, 0.38889],
    10217: [0.25, 0.75, 0, 0, 0.38889],
    10222: [0.244, 0.744, 0, 0, 0.412],
    10223: [0.244, 0.745, 0, 0, 0.412],
    10229: [0.011, 0.511, 0, 0, 1.609],
    10230: [0.011, 0.511, 0, 0, 1.638],
    10231: [0.011, 0.511, 0, 0, 1.859],
    10232: [0.024, 0.525, 0, 0, 1.609],
    10233: [0.024, 0.525, 0, 0, 1.638],
    10234: [0.024, 0.525, 0, 0, 1.858],
    10236: [0.011, 0.511, 0, 0, 1.638],
    10815: [0, 0.68333, 0, 0, 0.75],
    10927: [0.13597, 0.63597, 0, 0, 0.77778],
    10928: [0.13597, 0.63597, 0, 0, 0.77778],
    57376: [0.19444, 0.69444, 0, 0, 0]
  },
  "Math-BoldItalic": {
    32: [0, 0, 0, 0, 0.25],
    48: [0, 0.44444, 0, 0, 0.575],
    49: [0, 0.44444, 0, 0, 0.575],
    50: [0, 0.44444, 0, 0, 0.575],
    51: [0.19444, 0.44444, 0, 0, 0.575],
    52: [0.19444, 0.44444, 0, 0, 0.575],
    53: [0.19444, 0.44444, 0, 0, 0.575],
    54: [0, 0.64444, 0, 0, 0.575],
    55: [0.19444, 0.44444, 0, 0, 0.575],
    56: [0, 0.64444, 0, 0, 0.575],
    57: [0.19444, 0.44444, 0, 0, 0.575],
    65: [0, 0.68611, 0, 0, 0.86944],
    66: [0, 0.68611, 0.04835, 0, 0.8664],
    67: [0, 0.68611, 0.06979, 0, 0.81694],
    68: [0, 0.68611, 0.03194, 0, 0.93812],
    69: [0, 0.68611, 0.05451, 0, 0.81007],
    70: [0, 0.68611, 0.15972, 0, 0.68889],
    71: [0, 0.68611, 0, 0, 0.88673],
    72: [0, 0.68611, 0.08229, 0, 0.98229],
    73: [0, 0.68611, 0.07778, 0, 0.51111],
    74: [0, 0.68611, 0.10069, 0, 0.63125],
    75: [0, 0.68611, 0.06979, 0, 0.97118],
    76: [0, 0.68611, 0, 0, 0.75555],
    77: [0, 0.68611, 0.11424, 0, 1.14201],
    78: [0, 0.68611, 0.11424, 0, 0.95034],
    79: [0, 0.68611, 0.03194, 0, 0.83666],
    80: [0, 0.68611, 0.15972, 0, 0.72309],
    81: [0.19444, 0.68611, 0, 0, 0.86861],
    82: [0, 0.68611, 421e-5, 0, 0.87235],
    83: [0, 0.68611, 0.05382, 0, 0.69271],
    84: [0, 0.68611, 0.15972, 0, 0.63663],
    85: [0, 0.68611, 0.11424, 0, 0.80027],
    86: [0, 0.68611, 0.25555, 0, 0.67778],
    87: [0, 0.68611, 0.15972, 0, 1.09305],
    88: [0, 0.68611, 0.07778, 0, 0.94722],
    89: [0, 0.68611, 0.25555, 0, 0.67458],
    90: [0, 0.68611, 0.06979, 0, 0.77257],
    97: [0, 0.44444, 0, 0, 0.63287],
    98: [0, 0.69444, 0, 0, 0.52083],
    99: [0, 0.44444, 0, 0, 0.51342],
    100: [0, 0.69444, 0, 0, 0.60972],
    101: [0, 0.44444, 0, 0, 0.55361],
    102: [0.19444, 0.69444, 0.11042, 0, 0.56806],
    103: [0.19444, 0.44444, 0.03704, 0, 0.5449],
    104: [0, 0.69444, 0, 0, 0.66759],
    105: [0, 0.69326, 0, 0, 0.4048],
    106: [0.19444, 0.69326, 0.0622, 0, 0.47083],
    107: [0, 0.69444, 0.01852, 0, 0.6037],
    108: [0, 0.69444, 88e-4, 0, 0.34815],
    109: [0, 0.44444, 0, 0, 1.0324],
    110: [0, 0.44444, 0, 0, 0.71296],
    111: [0, 0.44444, 0, 0, 0.58472],
    112: [0.19444, 0.44444, 0, 0, 0.60092],
    113: [0.19444, 0.44444, 0.03704, 0, 0.54213],
    114: [0, 0.44444, 0.03194, 0, 0.5287],
    115: [0, 0.44444, 0, 0, 0.53125],
    116: [0, 0.63492, 0, 0, 0.41528],
    117: [0, 0.44444, 0, 0, 0.68102],
    118: [0, 0.44444, 0.03704, 0, 0.56666],
    119: [0, 0.44444, 0.02778, 0, 0.83148],
    120: [0, 0.44444, 0, 0, 0.65903],
    121: [0.19444, 0.44444, 0.03704, 0, 0.59028],
    122: [0, 0.44444, 0.04213, 0, 0.55509],
    160: [0, 0, 0, 0, 0.25],
    915: [0, 0.68611, 0.15972, 0, 0.65694],
    916: [0, 0.68611, 0, 0, 0.95833],
    920: [0, 0.68611, 0.03194, 0, 0.86722],
    923: [0, 0.68611, 0, 0, 0.80555],
    926: [0, 0.68611, 0.07458, 0, 0.84125],
    928: [0, 0.68611, 0.08229, 0, 0.98229],
    931: [0, 0.68611, 0.05451, 0, 0.88507],
    933: [0, 0.68611, 0.15972, 0, 0.67083],
    934: [0, 0.68611, 0, 0, 0.76666],
    936: [0, 0.68611, 0.11653, 0, 0.71402],
    937: [0, 0.68611, 0.04835, 0, 0.8789],
    945: [0, 0.44444, 0, 0, 0.76064],
    946: [0.19444, 0.69444, 0.03403, 0, 0.65972],
    947: [0.19444, 0.44444, 0.06389, 0, 0.59003],
    948: [0, 0.69444, 0.03819, 0, 0.52222],
    949: [0, 0.44444, 0, 0, 0.52882],
    950: [0.19444, 0.69444, 0.06215, 0, 0.50833],
    951: [0.19444, 0.44444, 0.03704, 0, 0.6],
    952: [0, 0.69444, 0.03194, 0, 0.5618],
    953: [0, 0.44444, 0, 0, 0.41204],
    954: [0, 0.44444, 0, 0, 0.66759],
    955: [0, 0.69444, 0, 0, 0.67083],
    956: [0.19444, 0.44444, 0, 0, 0.70787],
    957: [0, 0.44444, 0.06898, 0, 0.57685],
    958: [0.19444, 0.69444, 0.03021, 0, 0.50833],
    959: [0, 0.44444, 0, 0, 0.58472],
    960: [0, 0.44444, 0.03704, 0, 0.68241],
    961: [0.19444, 0.44444, 0, 0, 0.6118],
    962: [0.09722, 0.44444, 0.07917, 0, 0.42361],
    963: [0, 0.44444, 0.03704, 0, 0.68588],
    964: [0, 0.44444, 0.13472, 0, 0.52083],
    965: [0, 0.44444, 0.03704, 0, 0.63055],
    966: [0.19444, 0.44444, 0, 0, 0.74722],
    967: [0.19444, 0.44444, 0, 0, 0.71805],
    968: [0.19444, 0.69444, 0.03704, 0, 0.75833],
    969: [0, 0.44444, 0.03704, 0, 0.71782],
    977: [0, 0.69444, 0, 0, 0.69155],
    981: [0.19444, 0.69444, 0, 0, 0.7125],
    982: [0, 0.44444, 0.03194, 0, 0.975],
    1009: [0.19444, 0.44444, 0, 0, 0.6118],
    1013: [0, 0.44444, 0, 0, 0.48333],
    57649: [0, 0.44444, 0, 0, 0.39352],
    57911: [0.19444, 0.44444, 0, 0, 0.43889]
  },
  "Math-Italic": {
    32: [0, 0, 0, 0, 0.25],
    48: [0, 0.43056, 0, 0, 0.5],
    49: [0, 0.43056, 0, 0, 0.5],
    50: [0, 0.43056, 0, 0, 0.5],
    51: [0.19444, 0.43056, 0, 0, 0.5],
    52: [0.19444, 0.43056, 0, 0, 0.5],
    53: [0.19444, 0.43056, 0, 0, 0.5],
    54: [0, 0.64444, 0, 0, 0.5],
    55: [0.19444, 0.43056, 0, 0, 0.5],
    56: [0, 0.64444, 0, 0, 0.5],
    57: [0.19444, 0.43056, 0, 0, 0.5],
    65: [0, 0.68333, 0, 0.13889, 0.75],
    66: [0, 0.68333, 0.05017, 0.08334, 0.75851],
    67: [0, 0.68333, 0.07153, 0.08334, 0.71472],
    68: [0, 0.68333, 0.02778, 0.05556, 0.82792],
    69: [0, 0.68333, 0.05764, 0.08334, 0.7382],
    70: [0, 0.68333, 0.13889, 0.08334, 0.64306],
    71: [0, 0.68333, 0, 0.08334, 0.78625],
    72: [0, 0.68333, 0.08125, 0.05556, 0.83125],
    73: [0, 0.68333, 0.07847, 0.11111, 0.43958],
    74: [0, 0.68333, 0.09618, 0.16667, 0.55451],
    75: [0, 0.68333, 0.07153, 0.05556, 0.84931],
    76: [0, 0.68333, 0, 0.02778, 0.68056],
    77: [0, 0.68333, 0.10903, 0.08334, 0.97014],
    78: [0, 0.68333, 0.10903, 0.08334, 0.80347],
    79: [0, 0.68333, 0.02778, 0.08334, 0.76278],
    80: [0, 0.68333, 0.13889, 0.08334, 0.64201],
    81: [0.19444, 0.68333, 0, 0.08334, 0.79056],
    82: [0, 0.68333, 773e-5, 0.08334, 0.75929],
    83: [0, 0.68333, 0.05764, 0.08334, 0.6132],
    84: [0, 0.68333, 0.13889, 0.08334, 0.58438],
    85: [0, 0.68333, 0.10903, 0.02778, 0.68278],
    86: [0, 0.68333, 0.22222, 0, 0.58333],
    87: [0, 0.68333, 0.13889, 0, 0.94445],
    88: [0, 0.68333, 0.07847, 0.08334, 0.82847],
    89: [0, 0.68333, 0.22222, 0, 0.58056],
    90: [0, 0.68333, 0.07153, 0.08334, 0.68264],
    97: [0, 0.43056, 0, 0, 0.52859],
    98: [0, 0.69444, 0, 0, 0.42917],
    99: [0, 0.43056, 0, 0.05556, 0.43276],
    100: [0, 0.69444, 0, 0.16667, 0.52049],
    101: [0, 0.43056, 0, 0.05556, 0.46563],
    102: [0.19444, 0.69444, 0.10764, 0.16667, 0.48959],
    103: [0.19444, 0.43056, 0.03588, 0.02778, 0.47697],
    104: [0, 0.69444, 0, 0, 0.57616],
    105: [0, 0.65952, 0, 0, 0.34451],
    106: [0.19444, 0.65952, 0.05724, 0, 0.41181],
    107: [0, 0.69444, 0.03148, 0, 0.5206],
    108: [0, 0.69444, 0.01968, 0.08334, 0.29838],
    109: [0, 0.43056, 0, 0, 0.87801],
    110: [0, 0.43056, 0, 0, 0.60023],
    111: [0, 0.43056, 0, 0.05556, 0.48472],
    112: [0.19444, 0.43056, 0, 0.08334, 0.50313],
    113: [0.19444, 0.43056, 0.03588, 0.08334, 0.44641],
    114: [0, 0.43056, 0.02778, 0.05556, 0.45116],
    115: [0, 0.43056, 0, 0.05556, 0.46875],
    116: [0, 0.61508, 0, 0.08334, 0.36111],
    117: [0, 0.43056, 0, 0.02778, 0.57246],
    118: [0, 0.43056, 0.03588, 0.02778, 0.48472],
    119: [0, 0.43056, 0.02691, 0.08334, 0.71592],
    120: [0, 0.43056, 0, 0.02778, 0.57153],
    121: [0.19444, 0.43056, 0.03588, 0.05556, 0.49028],
    122: [0, 0.43056, 0.04398, 0.05556, 0.46505],
    160: [0, 0, 0, 0, 0.25],
    915: [0, 0.68333, 0.13889, 0.08334, 0.61528],
    916: [0, 0.68333, 0, 0.16667, 0.83334],
    920: [0, 0.68333, 0.02778, 0.08334, 0.76278],
    923: [0, 0.68333, 0, 0.16667, 0.69445],
    926: [0, 0.68333, 0.07569, 0.08334, 0.74236],
    928: [0, 0.68333, 0.08125, 0.05556, 0.83125],
    931: [0, 0.68333, 0.05764, 0.08334, 0.77986],
    933: [0, 0.68333, 0.13889, 0.05556, 0.58333],
    934: [0, 0.68333, 0, 0.08334, 0.66667],
    936: [0, 0.68333, 0.11, 0.05556, 0.61222],
    937: [0, 0.68333, 0.05017, 0.08334, 0.7724],
    945: [0, 0.43056, 37e-4, 0.02778, 0.6397],
    946: [0.19444, 0.69444, 0.05278, 0.08334, 0.56563],
    947: [0.19444, 0.43056, 0.05556, 0, 0.51773],
    948: [0, 0.69444, 0.03785, 0.05556, 0.44444],
    949: [0, 0.43056, 0, 0.08334, 0.46632],
    950: [0.19444, 0.69444, 0.07378, 0.08334, 0.4375],
    951: [0.19444, 0.43056, 0.03588, 0.05556, 0.49653],
    952: [0, 0.69444, 0.02778, 0.08334, 0.46944],
    953: [0, 0.43056, 0, 0.05556, 0.35394],
    954: [0, 0.43056, 0, 0, 0.57616],
    955: [0, 0.69444, 0, 0, 0.58334],
    956: [0.19444, 0.43056, 0, 0.02778, 0.60255],
    957: [0, 0.43056, 0.06366, 0.02778, 0.49398],
    958: [0.19444, 0.69444, 0.04601, 0.11111, 0.4375],
    959: [0, 0.43056, 0, 0.05556, 0.48472],
    960: [0, 0.43056, 0.03588, 0, 0.57003],
    961: [0.19444, 0.43056, 0, 0.08334, 0.51702],
    962: [0.09722, 0.43056, 0.07986, 0.08334, 0.36285],
    963: [0, 0.43056, 0.03588, 0, 0.57141],
    964: [0, 0.43056, 0.1132, 0.02778, 0.43715],
    965: [0, 0.43056, 0.03588, 0.02778, 0.54028],
    966: [0.19444, 0.43056, 0, 0.08334, 0.65417],
    967: [0.19444, 0.43056, 0, 0.05556, 0.62569],
    968: [0.19444, 0.69444, 0.03588, 0.11111, 0.65139],
    969: [0, 0.43056, 0.03588, 0, 0.62245],
    977: [0, 0.69444, 0, 0.08334, 0.59144],
    981: [0.19444, 0.69444, 0, 0.08334, 0.59583],
    982: [0, 0.43056, 0.02778, 0, 0.82813],
    1009: [0.19444, 0.43056, 0, 0.08334, 0.51702],
    1013: [0, 0.43056, 0, 0.05556, 0.4059],
    57649: [0, 0.43056, 0, 0.02778, 0.32246],
    57911: [0.19444, 0.43056, 0, 0.08334, 0.38403]
  },
  "SansSerif-Bold": {
    32: [0, 0, 0, 0, 0.25],
    33: [0, 0.69444, 0, 0, 0.36667],
    34: [0, 0.69444, 0, 0, 0.55834],
    35: [0.19444, 0.69444, 0, 0, 0.91667],
    36: [0.05556, 0.75, 0, 0, 0.55],
    37: [0.05556, 0.75, 0, 0, 1.02912],
    38: [0, 0.69444, 0, 0, 0.83056],
    39: [0, 0.69444, 0, 0, 0.30556],
    40: [0.25, 0.75, 0, 0, 0.42778],
    41: [0.25, 0.75, 0, 0, 0.42778],
    42: [0, 0.75, 0, 0, 0.55],
    43: [0.11667, 0.61667, 0, 0, 0.85556],
    44: [0.10556, 0.13056, 0, 0, 0.30556],
    45: [0, 0.45833, 0, 0, 0.36667],
    46: [0, 0.13056, 0, 0, 0.30556],
    47: [0.25, 0.75, 0, 0, 0.55],
    48: [0, 0.69444, 0, 0, 0.55],
    49: [0, 0.69444, 0, 0, 0.55],
    50: [0, 0.69444, 0, 0, 0.55],
    51: [0, 0.69444, 0, 0, 0.55],
    52: [0, 0.69444, 0, 0, 0.55],
    53: [0, 0.69444, 0, 0, 0.55],
    54: [0, 0.69444, 0, 0, 0.55],
    55: [0, 0.69444, 0, 0, 0.55],
    56: [0, 0.69444, 0, 0, 0.55],
    57: [0, 0.69444, 0, 0, 0.55],
    58: [0, 0.45833, 0, 0, 0.30556],
    59: [0.10556, 0.45833, 0, 0, 0.30556],
    61: [-0.09375, 0.40625, 0, 0, 0.85556],
    63: [0, 0.69444, 0, 0, 0.51945],
    64: [0, 0.69444, 0, 0, 0.73334],
    65: [0, 0.69444, 0, 0, 0.73334],
    66: [0, 0.69444, 0, 0, 0.73334],
    67: [0, 0.69444, 0, 0, 0.70278],
    68: [0, 0.69444, 0, 0, 0.79445],
    69: [0, 0.69444, 0, 0, 0.64167],
    70: [0, 0.69444, 0, 0, 0.61111],
    71: [0, 0.69444, 0, 0, 0.73334],
    72: [0, 0.69444, 0, 0, 0.79445],
    73: [0, 0.69444, 0, 0, 0.33056],
    74: [0, 0.69444, 0, 0, 0.51945],
    75: [0, 0.69444, 0, 0, 0.76389],
    76: [0, 0.69444, 0, 0, 0.58056],
    77: [0, 0.69444, 0, 0, 0.97778],
    78: [0, 0.69444, 0, 0, 0.79445],
    79: [0, 0.69444, 0, 0, 0.79445],
    80: [0, 0.69444, 0, 0, 0.70278],
    81: [0.10556, 0.69444, 0, 0, 0.79445],
    82: [0, 0.69444, 0, 0, 0.70278],
    83: [0, 0.69444, 0, 0, 0.61111],
    84: [0, 0.69444, 0, 0, 0.73334],
    85: [0, 0.69444, 0, 0, 0.76389],
    86: [0, 0.69444, 0.01528, 0, 0.73334],
    87: [0, 0.69444, 0.01528, 0, 1.03889],
    88: [0, 0.69444, 0, 0, 0.73334],
    89: [0, 0.69444, 0.0275, 0, 0.73334],
    90: [0, 0.69444, 0, 0, 0.67223],
    91: [0.25, 0.75, 0, 0, 0.34306],
    93: [0.25, 0.75, 0, 0, 0.34306],
    94: [0, 0.69444, 0, 0, 0.55],
    95: [0.35, 0.10833, 0.03056, 0, 0.55],
    97: [0, 0.45833, 0, 0, 0.525],
    98: [0, 0.69444, 0, 0, 0.56111],
    99: [0, 0.45833, 0, 0, 0.48889],
    100: [0, 0.69444, 0, 0, 0.56111],
    101: [0, 0.45833, 0, 0, 0.51111],
    102: [0, 0.69444, 0.07639, 0, 0.33611],
    103: [0.19444, 0.45833, 0.01528, 0, 0.55],
    104: [0, 0.69444, 0, 0, 0.56111],
    105: [0, 0.69444, 0, 0, 0.25556],
    106: [0.19444, 0.69444, 0, 0, 0.28611],
    107: [0, 0.69444, 0, 0, 0.53056],
    108: [0, 0.69444, 0, 0, 0.25556],
    109: [0, 0.45833, 0, 0, 0.86667],
    110: [0, 0.45833, 0, 0, 0.56111],
    111: [0, 0.45833, 0, 0, 0.55],
    112: [0.19444, 0.45833, 0, 0, 0.56111],
    113: [0.19444, 0.45833, 0, 0, 0.56111],
    114: [0, 0.45833, 0.01528, 0, 0.37222],
    115: [0, 0.45833, 0, 0, 0.42167],
    116: [0, 0.58929, 0, 0, 0.40417],
    117: [0, 0.45833, 0, 0, 0.56111],
    118: [0, 0.45833, 0.01528, 0, 0.5],
    119: [0, 0.45833, 0.01528, 0, 0.74445],
    120: [0, 0.45833, 0, 0, 0.5],
    121: [0.19444, 0.45833, 0.01528, 0, 0.5],
    122: [0, 0.45833, 0, 0, 0.47639],
    126: [0.35, 0.34444, 0, 0, 0.55],
    160: [0, 0, 0, 0, 0.25],
    168: [0, 0.69444, 0, 0, 0.55],
    176: [0, 0.69444, 0, 0, 0.73334],
    180: [0, 0.69444, 0, 0, 0.55],
    184: [0.17014, 0, 0, 0, 0.48889],
    305: [0, 0.45833, 0, 0, 0.25556],
    567: [0.19444, 0.45833, 0, 0, 0.28611],
    710: [0, 0.69444, 0, 0, 0.55],
    711: [0, 0.63542, 0, 0, 0.55],
    713: [0, 0.63778, 0, 0, 0.55],
    728: [0, 0.69444, 0, 0, 0.55],
    729: [0, 0.69444, 0, 0, 0.30556],
    730: [0, 0.69444, 0, 0, 0.73334],
    732: [0, 0.69444, 0, 0, 0.55],
    733: [0, 0.69444, 0, 0, 0.55],
    915: [0, 0.69444, 0, 0, 0.58056],
    916: [0, 0.69444, 0, 0, 0.91667],
    920: [0, 0.69444, 0, 0, 0.85556],
    923: [0, 0.69444, 0, 0, 0.67223],
    926: [0, 0.69444, 0, 0, 0.73334],
    928: [0, 0.69444, 0, 0, 0.79445],
    931: [0, 0.69444, 0, 0, 0.79445],
    933: [0, 0.69444, 0, 0, 0.85556],
    934: [0, 0.69444, 0, 0, 0.79445],
    936: [0, 0.69444, 0, 0, 0.85556],
    937: [0, 0.69444, 0, 0, 0.79445],
    8211: [0, 0.45833, 0.03056, 0, 0.55],
    8212: [0, 0.45833, 0.03056, 0, 1.10001],
    8216: [0, 0.69444, 0, 0, 0.30556],
    8217: [0, 0.69444, 0, 0, 0.30556],
    8220: [0, 0.69444, 0, 0, 0.55834],
    8221: [0, 0.69444, 0, 0, 0.55834]
  },
  "SansSerif-Italic": {
    32: [0, 0, 0, 0, 0.25],
    33: [0, 0.69444, 0.05733, 0, 0.31945],
    34: [0, 0.69444, 316e-5, 0, 0.5],
    35: [0.19444, 0.69444, 0.05087, 0, 0.83334],
    36: [0.05556, 0.75, 0.11156, 0, 0.5],
    37: [0.05556, 0.75, 0.03126, 0, 0.83334],
    38: [0, 0.69444, 0.03058, 0, 0.75834],
    39: [0, 0.69444, 0.07816, 0, 0.27778],
    40: [0.25, 0.75, 0.13164, 0, 0.38889],
    41: [0.25, 0.75, 0.02536, 0, 0.38889],
    42: [0, 0.75, 0.11775, 0, 0.5],
    43: [0.08333, 0.58333, 0.02536, 0, 0.77778],
    44: [0.125, 0.08333, 0, 0, 0.27778],
    45: [0, 0.44444, 0.01946, 0, 0.33333],
    46: [0, 0.08333, 0, 0, 0.27778],
    47: [0.25, 0.75, 0.13164, 0, 0.5],
    48: [0, 0.65556, 0.11156, 0, 0.5],
    49: [0, 0.65556, 0.11156, 0, 0.5],
    50: [0, 0.65556, 0.11156, 0, 0.5],
    51: [0, 0.65556, 0.11156, 0, 0.5],
    52: [0, 0.65556, 0.11156, 0, 0.5],
    53: [0, 0.65556, 0.11156, 0, 0.5],
    54: [0, 0.65556, 0.11156, 0, 0.5],
    55: [0, 0.65556, 0.11156, 0, 0.5],
    56: [0, 0.65556, 0.11156, 0, 0.5],
    57: [0, 0.65556, 0.11156, 0, 0.5],
    58: [0, 0.44444, 0.02502, 0, 0.27778],
    59: [0.125, 0.44444, 0.02502, 0, 0.27778],
    61: [-0.13, 0.37, 0.05087, 0, 0.77778],
    63: [0, 0.69444, 0.11809, 0, 0.47222],
    64: [0, 0.69444, 0.07555, 0, 0.66667],
    65: [0, 0.69444, 0, 0, 0.66667],
    66: [0, 0.69444, 0.08293, 0, 0.66667],
    67: [0, 0.69444, 0.11983, 0, 0.63889],
    68: [0, 0.69444, 0.07555, 0, 0.72223],
    69: [0, 0.69444, 0.11983, 0, 0.59722],
    70: [0, 0.69444, 0.13372, 0, 0.56945],
    71: [0, 0.69444, 0.11983, 0, 0.66667],
    72: [0, 0.69444, 0.08094, 0, 0.70834],
    73: [0, 0.69444, 0.13372, 0, 0.27778],
    74: [0, 0.69444, 0.08094, 0, 0.47222],
    75: [0, 0.69444, 0.11983, 0, 0.69445],
    76: [0, 0.69444, 0, 0, 0.54167],
    77: [0, 0.69444, 0.08094, 0, 0.875],
    78: [0, 0.69444, 0.08094, 0, 0.70834],
    79: [0, 0.69444, 0.07555, 0, 0.73611],
    80: [0, 0.69444, 0.08293, 0, 0.63889],
    81: [0.125, 0.69444, 0.07555, 0, 0.73611],
    82: [0, 0.69444, 0.08293, 0, 0.64584],
    83: [0, 0.69444, 0.09205, 0, 0.55556],
    84: [0, 0.69444, 0.13372, 0, 0.68056],
    85: [0, 0.69444, 0.08094, 0, 0.6875],
    86: [0, 0.69444, 0.1615, 0, 0.66667],
    87: [0, 0.69444, 0.1615, 0, 0.94445],
    88: [0, 0.69444, 0.13372, 0, 0.66667],
    89: [0, 0.69444, 0.17261, 0, 0.66667],
    90: [0, 0.69444, 0.11983, 0, 0.61111],
    91: [0.25, 0.75, 0.15942, 0, 0.28889],
    93: [0.25, 0.75, 0.08719, 0, 0.28889],
    94: [0, 0.69444, 0.0799, 0, 0.5],
    95: [0.35, 0.09444, 0.08616, 0, 0.5],
    97: [0, 0.44444, 981e-5, 0, 0.48056],
    98: [0, 0.69444, 0.03057, 0, 0.51667],
    99: [0, 0.44444, 0.08336, 0, 0.44445],
    100: [0, 0.69444, 0.09483, 0, 0.51667],
    101: [0, 0.44444, 0.06778, 0, 0.44445],
    102: [0, 0.69444, 0.21705, 0, 0.30556],
    103: [0.19444, 0.44444, 0.10836, 0, 0.5],
    104: [0, 0.69444, 0.01778, 0, 0.51667],
    105: [0, 0.67937, 0.09718, 0, 0.23889],
    106: [0.19444, 0.67937, 0.09162, 0, 0.26667],
    107: [0, 0.69444, 0.08336, 0, 0.48889],
    108: [0, 0.69444, 0.09483, 0, 0.23889],
    109: [0, 0.44444, 0.01778, 0, 0.79445],
    110: [0, 0.44444, 0.01778, 0, 0.51667],
    111: [0, 0.44444, 0.06613, 0, 0.5],
    112: [0.19444, 0.44444, 0.0389, 0, 0.51667],
    113: [0.19444, 0.44444, 0.04169, 0, 0.51667],
    114: [0, 0.44444, 0.10836, 0, 0.34167],
    115: [0, 0.44444, 0.0778, 0, 0.38333],
    116: [0, 0.57143, 0.07225, 0, 0.36111],
    117: [0, 0.44444, 0.04169, 0, 0.51667],
    118: [0, 0.44444, 0.10836, 0, 0.46111],
    119: [0, 0.44444, 0.10836, 0, 0.68334],
    120: [0, 0.44444, 0.09169, 0, 0.46111],
    121: [0.19444, 0.44444, 0.10836, 0, 0.46111],
    122: [0, 0.44444, 0.08752, 0, 0.43472],
    126: [0.35, 0.32659, 0.08826, 0, 0.5],
    160: [0, 0, 0, 0, 0.25],
    168: [0, 0.67937, 0.06385, 0, 0.5],
    176: [0, 0.69444, 0, 0, 0.73752],
    184: [0.17014, 0, 0, 0, 0.44445],
    305: [0, 0.44444, 0.04169, 0, 0.23889],
    567: [0.19444, 0.44444, 0.04169, 0, 0.26667],
    710: [0, 0.69444, 0.0799, 0, 0.5],
    711: [0, 0.63194, 0.08432, 0, 0.5],
    713: [0, 0.60889, 0.08776, 0, 0.5],
    714: [0, 0.69444, 0.09205, 0, 0.5],
    715: [0, 0.69444, 0, 0, 0.5],
    728: [0, 0.69444, 0.09483, 0, 0.5],
    729: [0, 0.67937, 0.07774, 0, 0.27778],
    730: [0, 0.69444, 0, 0, 0.73752],
    732: [0, 0.67659, 0.08826, 0, 0.5],
    733: [0, 0.69444, 0.09205, 0, 0.5],
    915: [0, 0.69444, 0.13372, 0, 0.54167],
    916: [0, 0.69444, 0, 0, 0.83334],
    920: [0, 0.69444, 0.07555, 0, 0.77778],
    923: [0, 0.69444, 0, 0, 0.61111],
    926: [0, 0.69444, 0.12816, 0, 0.66667],
    928: [0, 0.69444, 0.08094, 0, 0.70834],
    931: [0, 0.69444, 0.11983, 0, 0.72222],
    933: [0, 0.69444, 0.09031, 0, 0.77778],
    934: [0, 0.69444, 0.04603, 0, 0.72222],
    936: [0, 0.69444, 0.09031, 0, 0.77778],
    937: [0, 0.69444, 0.08293, 0, 0.72222],
    8211: [0, 0.44444, 0.08616, 0, 0.5],
    8212: [0, 0.44444, 0.08616, 0, 1],
    8216: [0, 0.69444, 0.07816, 0, 0.27778],
    8217: [0, 0.69444, 0.07816, 0, 0.27778],
    8220: [0, 0.69444, 0.14205, 0, 0.5],
    8221: [0, 0.69444, 316e-5, 0, 0.5]
  },
  "SansSerif-Regular": {
    32: [0, 0, 0, 0, 0.25],
    33: [0, 0.69444, 0, 0, 0.31945],
    34: [0, 0.69444, 0, 0, 0.5],
    35: [0.19444, 0.69444, 0, 0, 0.83334],
    36: [0.05556, 0.75, 0, 0, 0.5],
    37: [0.05556, 0.75, 0, 0, 0.83334],
    38: [0, 0.69444, 0, 0, 0.75834],
    39: [0, 0.69444, 0, 0, 0.27778],
    40: [0.25, 0.75, 0, 0, 0.38889],
    41: [0.25, 0.75, 0, 0, 0.38889],
    42: [0, 0.75, 0, 0, 0.5],
    43: [0.08333, 0.58333, 0, 0, 0.77778],
    44: [0.125, 0.08333, 0, 0, 0.27778],
    45: [0, 0.44444, 0, 0, 0.33333],
    46: [0, 0.08333, 0, 0, 0.27778],
    47: [0.25, 0.75, 0, 0, 0.5],
    48: [0, 0.65556, 0, 0, 0.5],
    49: [0, 0.65556, 0, 0, 0.5],
    50: [0, 0.65556, 0, 0, 0.5],
    51: [0, 0.65556, 0, 0, 0.5],
    52: [0, 0.65556, 0, 0, 0.5],
    53: [0, 0.65556, 0, 0, 0.5],
    54: [0, 0.65556, 0, 0, 0.5],
    55: [0, 0.65556, 0, 0, 0.5],
    56: [0, 0.65556, 0, 0, 0.5],
    57: [0, 0.65556, 0, 0, 0.5],
    58: [0, 0.44444, 0, 0, 0.27778],
    59: [0.125, 0.44444, 0, 0, 0.27778],
    61: [-0.13, 0.37, 0, 0, 0.77778],
    63: [0, 0.69444, 0, 0, 0.47222],
    64: [0, 0.69444, 0, 0, 0.66667],
    65: [0, 0.69444, 0, 0, 0.66667],
    66: [0, 0.69444, 0, 0, 0.66667],
    67: [0, 0.69444, 0, 0, 0.63889],
    68: [0, 0.69444, 0, 0, 0.72223],
    69: [0, 0.69444, 0, 0, 0.59722],
    70: [0, 0.69444, 0, 0, 0.56945],
    71: [0, 0.69444, 0, 0, 0.66667],
    72: [0, 0.69444, 0, 0, 0.70834],
    73: [0, 0.69444, 0, 0, 0.27778],
    74: [0, 0.69444, 0, 0, 0.47222],
    75: [0, 0.69444, 0, 0, 0.69445],
    76: [0, 0.69444, 0, 0, 0.54167],
    77: [0, 0.69444, 0, 0, 0.875],
    78: [0, 0.69444, 0, 0, 0.70834],
    79: [0, 0.69444, 0, 0, 0.73611],
    80: [0, 0.69444, 0, 0, 0.63889],
    81: [0.125, 0.69444, 0, 0, 0.73611],
    82: [0, 0.69444, 0, 0, 0.64584],
    83: [0, 0.69444, 0, 0, 0.55556],
    84: [0, 0.69444, 0, 0, 0.68056],
    85: [0, 0.69444, 0, 0, 0.6875],
    86: [0, 0.69444, 0.01389, 0, 0.66667],
    87: [0, 0.69444, 0.01389, 0, 0.94445],
    88: [0, 0.69444, 0, 0, 0.66667],
    89: [0, 0.69444, 0.025, 0, 0.66667],
    90: [0, 0.69444, 0, 0, 0.61111],
    91: [0.25, 0.75, 0, 0, 0.28889],
    93: [0.25, 0.75, 0, 0, 0.28889],
    94: [0, 0.69444, 0, 0, 0.5],
    95: [0.35, 0.09444, 0.02778, 0, 0.5],
    97: [0, 0.44444, 0, 0, 0.48056],
    98: [0, 0.69444, 0, 0, 0.51667],
    99: [0, 0.44444, 0, 0, 0.44445],
    100: [0, 0.69444, 0, 0, 0.51667],
    101: [0, 0.44444, 0, 0, 0.44445],
    102: [0, 0.69444, 0.06944, 0, 0.30556],
    103: [0.19444, 0.44444, 0.01389, 0, 0.5],
    104: [0, 0.69444, 0, 0, 0.51667],
    105: [0, 0.67937, 0, 0, 0.23889],
    106: [0.19444, 0.67937, 0, 0, 0.26667],
    107: [0, 0.69444, 0, 0, 0.48889],
    108: [0, 0.69444, 0, 0, 0.23889],
    109: [0, 0.44444, 0, 0, 0.79445],
    110: [0, 0.44444, 0, 0, 0.51667],
    111: [0, 0.44444, 0, 0, 0.5],
    112: [0.19444, 0.44444, 0, 0, 0.51667],
    113: [0.19444, 0.44444, 0, 0, 0.51667],
    114: [0, 0.44444, 0.01389, 0, 0.34167],
    115: [0, 0.44444, 0, 0, 0.38333],
    116: [0, 0.57143, 0, 0, 0.36111],
    117: [0, 0.44444, 0, 0, 0.51667],
    118: [0, 0.44444, 0.01389, 0, 0.46111],
    119: [0, 0.44444, 0.01389, 0, 0.68334],
    120: [0, 0.44444, 0, 0, 0.46111],
    121: [0.19444, 0.44444, 0.01389, 0, 0.46111],
    122: [0, 0.44444, 0, 0, 0.43472],
    126: [0.35, 0.32659, 0, 0, 0.5],
    160: [0, 0, 0, 0, 0.25],
    168: [0, 0.67937, 0, 0, 0.5],
    176: [0, 0.69444, 0, 0, 0.66667],
    184: [0.17014, 0, 0, 0, 0.44445],
    305: [0, 0.44444, 0, 0, 0.23889],
    567: [0.19444, 0.44444, 0, 0, 0.26667],
    710: [0, 0.69444, 0, 0, 0.5],
    711: [0, 0.63194, 0, 0, 0.5],
    713: [0, 0.60889, 0, 0, 0.5],
    714: [0, 0.69444, 0, 0, 0.5],
    715: [0, 0.69444, 0, 0, 0.5],
    728: [0, 0.69444, 0, 0, 0.5],
    729: [0, 0.67937, 0, 0, 0.27778],
    730: [0, 0.69444, 0, 0, 0.66667],
    732: [0, 0.67659, 0, 0, 0.5],
    733: [0, 0.69444, 0, 0, 0.5],
    915: [0, 0.69444, 0, 0, 0.54167],
    916: [0, 0.69444, 0, 0, 0.83334],
    920: [0, 0.69444, 0, 0, 0.77778],
    923: [0, 0.69444, 0, 0, 0.61111],
    926: [0, 0.69444, 0, 0, 0.66667],
    928: [0, 0.69444, 0, 0, 0.70834],
    931: [0, 0.69444, 0, 0, 0.72222],
    933: [0, 0.69444, 0, 0, 0.77778],
    934: [0, 0.69444, 0, 0, 0.72222],
    936: [0, 0.69444, 0, 0, 0.77778],
    937: [0, 0.69444, 0, 0, 0.72222],
    8211: [0, 0.44444, 0.02778, 0, 0.5],
    8212: [0, 0.44444, 0.02778, 0, 1],
    8216: [0, 0.69444, 0, 0, 0.27778],
    8217: [0, 0.69444, 0, 0, 0.27778],
    8220: [0, 0.69444, 0, 0, 0.5],
    8221: [0, 0.69444, 0, 0, 0.5]
  },
  "Script-Regular": {
    32: [0, 0, 0, 0, 0.25],
    65: [0, 0.7, 0.22925, 0, 0.80253],
    66: [0, 0.7, 0.04087, 0, 0.90757],
    67: [0, 0.7, 0.1689, 0, 0.66619],
    68: [0, 0.7, 0.09371, 0, 0.77443],
    69: [0, 0.7, 0.18583, 0, 0.56162],
    70: [0, 0.7, 0.13634, 0, 0.89544],
    71: [0, 0.7, 0.17322, 0, 0.60961],
    72: [0, 0.7, 0.29694, 0, 0.96919],
    73: [0, 0.7, 0.19189, 0, 0.80907],
    74: [0.27778, 0.7, 0.19189, 0, 1.05159],
    75: [0, 0.7, 0.31259, 0, 0.91364],
    76: [0, 0.7, 0.19189, 0, 0.87373],
    77: [0, 0.7, 0.15981, 0, 1.08031],
    78: [0, 0.7, 0.3525, 0, 0.9015],
    79: [0, 0.7, 0.08078, 0, 0.73787],
    80: [0, 0.7, 0.08078, 0, 1.01262],
    81: [0, 0.7, 0.03305, 0, 0.88282],
    82: [0, 0.7, 0.06259, 0, 0.85],
    83: [0, 0.7, 0.19189, 0, 0.86767],
    84: [0, 0.7, 0.29087, 0, 0.74697],
    85: [0, 0.7, 0.25815, 0, 0.79996],
    86: [0, 0.7, 0.27523, 0, 0.62204],
    87: [0, 0.7, 0.27523, 0, 0.80532],
    88: [0, 0.7, 0.26006, 0, 0.94445],
    89: [0, 0.7, 0.2939, 0, 0.70961],
    90: [0, 0.7, 0.24037, 0, 0.8212],
    160: [0, 0, 0, 0, 0.25]
  },
  "Size1-Regular": {
    32: [0, 0, 0, 0, 0.25],
    40: [0.35001, 0.85, 0, 0, 0.45834],
    41: [0.35001, 0.85, 0, 0, 0.45834],
    47: [0.35001, 0.85, 0, 0, 0.57778],
    91: [0.35001, 0.85, 0, 0, 0.41667],
    92: [0.35001, 0.85, 0, 0, 0.57778],
    93: [0.35001, 0.85, 0, 0, 0.41667],
    123: [0.35001, 0.85, 0, 0, 0.58334],
    125: [0.35001, 0.85, 0, 0, 0.58334],
    160: [0, 0, 0, 0, 0.25],
    710: [0, 0.72222, 0, 0, 0.55556],
    732: [0, 0.72222, 0, 0, 0.55556],
    770: [0, 0.72222, 0, 0, 0.55556],
    771: [0, 0.72222, 0, 0, 0.55556],
    8214: [-99e-5, 0.601, 0, 0, 0.77778],
    8593: [1e-5, 0.6, 0, 0, 0.66667],
    8595: [1e-5, 0.6, 0, 0, 0.66667],
    8657: [1e-5, 0.6, 0, 0, 0.77778],
    8659: [1e-5, 0.6, 0, 0, 0.77778],
    8719: [0.25001, 0.75, 0, 0, 0.94445],
    8720: [0.25001, 0.75, 0, 0, 0.94445],
    8721: [0.25001, 0.75, 0, 0, 1.05556],
    8730: [0.35001, 0.85, 0, 0, 1],
    8739: [-599e-5, 0.606, 0, 0, 0.33333],
    8741: [-599e-5, 0.606, 0, 0, 0.55556],
    8747: [0.30612, 0.805, 0.19445, 0, 0.47222],
    8748: [0.306, 0.805, 0.19445, 0, 0.47222],
    8749: [0.306, 0.805, 0.19445, 0, 0.47222],
    8750: [0.30612, 0.805, 0.19445, 0, 0.47222],
    8896: [0.25001, 0.75, 0, 0, 0.83334],
    8897: [0.25001, 0.75, 0, 0, 0.83334],
    8898: [0.25001, 0.75, 0, 0, 0.83334],
    8899: [0.25001, 0.75, 0, 0, 0.83334],
    8968: [0.35001, 0.85, 0, 0, 0.47222],
    8969: [0.35001, 0.85, 0, 0, 0.47222],
    8970: [0.35001, 0.85, 0, 0, 0.47222],
    8971: [0.35001, 0.85, 0, 0, 0.47222],
    9168: [-99e-5, 0.601, 0, 0, 0.66667],
    10216: [0.35001, 0.85, 0, 0, 0.47222],
    10217: [0.35001, 0.85, 0, 0, 0.47222],
    10752: [0.25001, 0.75, 0, 0, 1.11111],
    10753: [0.25001, 0.75, 0, 0, 1.11111],
    10754: [0.25001, 0.75, 0, 0, 1.11111],
    10756: [0.25001, 0.75, 0, 0, 0.83334],
    10758: [0.25001, 0.75, 0, 0, 0.83334]
  },
  "Size2-Regular": {
    32: [0, 0, 0, 0, 0.25],
    40: [0.65002, 1.15, 0, 0, 0.59722],
    41: [0.65002, 1.15, 0, 0, 0.59722],
    47: [0.65002, 1.15, 0, 0, 0.81111],
    91: [0.65002, 1.15, 0, 0, 0.47222],
    92: [0.65002, 1.15, 0, 0, 0.81111],
    93: [0.65002, 1.15, 0, 0, 0.47222],
    123: [0.65002, 1.15, 0, 0, 0.66667],
    125: [0.65002, 1.15, 0, 0, 0.66667],
    160: [0, 0, 0, 0, 0.25],
    710: [0, 0.75, 0, 0, 1],
    732: [0, 0.75, 0, 0, 1],
    770: [0, 0.75, 0, 0, 1],
    771: [0, 0.75, 0, 0, 1],
    8719: [0.55001, 1.05, 0, 0, 1.27778],
    8720: [0.55001, 1.05, 0, 0, 1.27778],
    8721: [0.55001, 1.05, 0, 0, 1.44445],
    8730: [0.65002, 1.15, 0, 0, 1],
    8747: [0.86225, 1.36, 0.44445, 0, 0.55556],
    8748: [0.862, 1.36, 0.44445, 0, 0.55556],
    8749: [0.862, 1.36, 0.44445, 0, 0.55556],
    8750: [0.86225, 1.36, 0.44445, 0, 0.55556],
    8896: [0.55001, 1.05, 0, 0, 1.11111],
    8897: [0.55001, 1.05, 0, 0, 1.11111],
    8898: [0.55001, 1.05, 0, 0, 1.11111],
    8899: [0.55001, 1.05, 0, 0, 1.11111],
    8968: [0.65002, 1.15, 0, 0, 0.52778],
    8969: [0.65002, 1.15, 0, 0, 0.52778],
    8970: [0.65002, 1.15, 0, 0, 0.52778],
    8971: [0.65002, 1.15, 0, 0, 0.52778],
    10216: [0.65002, 1.15, 0, 0, 0.61111],
    10217: [0.65002, 1.15, 0, 0, 0.61111],
    10752: [0.55001, 1.05, 0, 0, 1.51112],
    10753: [0.55001, 1.05, 0, 0, 1.51112],
    10754: [0.55001, 1.05, 0, 0, 1.51112],
    10756: [0.55001, 1.05, 0, 0, 1.11111],
    10758: [0.55001, 1.05, 0, 0, 1.11111]
  },
  "Size3-Regular": {
    32: [0, 0, 0, 0, 0.25],
    40: [0.95003, 1.45, 0, 0, 0.73611],
    41: [0.95003, 1.45, 0, 0, 0.73611],
    47: [0.95003, 1.45, 0, 0, 1.04445],
    91: [0.95003, 1.45, 0, 0, 0.52778],
    92: [0.95003, 1.45, 0, 0, 1.04445],
    93: [0.95003, 1.45, 0, 0, 0.52778],
    123: [0.95003, 1.45, 0, 0, 0.75],
    125: [0.95003, 1.45, 0, 0, 0.75],
    160: [0, 0, 0, 0, 0.25],
    710: [0, 0.75, 0, 0, 1.44445],
    732: [0, 0.75, 0, 0, 1.44445],
    770: [0, 0.75, 0, 0, 1.44445],
    771: [0, 0.75, 0, 0, 1.44445],
    8730: [0.95003, 1.45, 0, 0, 1],
    8968: [0.95003, 1.45, 0, 0, 0.58334],
    8969: [0.95003, 1.45, 0, 0, 0.58334],
    8970: [0.95003, 1.45, 0, 0, 0.58334],
    8971: [0.95003, 1.45, 0, 0, 0.58334],
    10216: [0.95003, 1.45, 0, 0, 0.75],
    10217: [0.95003, 1.45, 0, 0, 0.75]
  },
  "Size4-Regular": {
    32: [0, 0, 0, 0, 0.25],
    40: [1.25003, 1.75, 0, 0, 0.79167],
    41: [1.25003, 1.75, 0, 0, 0.79167],
    47: [1.25003, 1.75, 0, 0, 1.27778],
    91: [1.25003, 1.75, 0, 0, 0.58334],
    92: [1.25003, 1.75, 0, 0, 1.27778],
    93: [1.25003, 1.75, 0, 0, 0.58334],
    123: [1.25003, 1.75, 0, 0, 0.80556],
    125: [1.25003, 1.75, 0, 0, 0.80556],
    160: [0, 0, 0, 0, 0.25],
    710: [0, 0.825, 0, 0, 1.8889],
    732: [0, 0.825, 0, 0, 1.8889],
    770: [0, 0.825, 0, 0, 1.8889],
    771: [0, 0.825, 0, 0, 1.8889],
    8730: [1.25003, 1.75, 0, 0, 1],
    8968: [1.25003, 1.75, 0, 0, 0.63889],
    8969: [1.25003, 1.75, 0, 0, 0.63889],
    8970: [1.25003, 1.75, 0, 0, 0.63889],
    8971: [1.25003, 1.75, 0, 0, 0.63889],
    9115: [0.64502, 1.155, 0, 0, 0.875],
    9116: [1e-5, 0.6, 0, 0, 0.875],
    9117: [0.64502, 1.155, 0, 0, 0.875],
    9118: [0.64502, 1.155, 0, 0, 0.875],
    9119: [1e-5, 0.6, 0, 0, 0.875],
    9120: [0.64502, 1.155, 0, 0, 0.875],
    9121: [0.64502, 1.155, 0, 0, 0.66667],
    9122: [-99e-5, 0.601, 0, 0, 0.66667],
    9123: [0.64502, 1.155, 0, 0, 0.66667],
    9124: [0.64502, 1.155, 0, 0, 0.66667],
    9125: [-99e-5, 0.601, 0, 0, 0.66667],
    9126: [0.64502, 1.155, 0, 0, 0.66667],
    9127: [1e-5, 0.9, 0, 0, 0.88889],
    9128: [0.65002, 1.15, 0, 0, 0.88889],
    9129: [0.90001, 0, 0, 0, 0.88889],
    9130: [0, 0.3, 0, 0, 0.88889],
    9131: [1e-5, 0.9, 0, 0, 0.88889],
    9132: [0.65002, 1.15, 0, 0, 0.88889],
    9133: [0.90001, 0, 0, 0, 0.88889],
    9143: [0.88502, 0.915, 0, 0, 1.05556],
    10216: [1.25003, 1.75, 0, 0, 0.80556],
    10217: [1.25003, 1.75, 0, 0, 0.80556],
    57344: [-499e-5, 0.605, 0, 0, 1.05556],
    57345: [-499e-5, 0.605, 0, 0, 1.05556],
    57680: [0, 0.12, 0, 0, 0.45],
    57681: [0, 0.12, 0, 0, 0.45],
    57682: [0, 0.12, 0, 0, 0.45],
    57683: [0, 0.12, 0, 0, 0.45]
  },
  "Typewriter-Regular": {
    32: [0, 0, 0, 0, 0.525],
    33: [0, 0.61111, 0, 0, 0.525],
    34: [0, 0.61111, 0, 0, 0.525],
    35: [0, 0.61111, 0, 0, 0.525],
    36: [0.08333, 0.69444, 0, 0, 0.525],
    37: [0.08333, 0.69444, 0, 0, 0.525],
    38: [0, 0.61111, 0, 0, 0.525],
    39: [0, 0.61111, 0, 0, 0.525],
    40: [0.08333, 0.69444, 0, 0, 0.525],
    41: [0.08333, 0.69444, 0, 0, 0.525],
    42: [0, 0.52083, 0, 0, 0.525],
    43: [-0.08056, 0.53055, 0, 0, 0.525],
    44: [0.13889, 0.125, 0, 0, 0.525],
    45: [-0.08056, 0.53055, 0, 0, 0.525],
    46: [0, 0.125, 0, 0, 0.525],
    47: [0.08333, 0.69444, 0, 0, 0.525],
    48: [0, 0.61111, 0, 0, 0.525],
    49: [0, 0.61111, 0, 0, 0.525],
    50: [0, 0.61111, 0, 0, 0.525],
    51: [0, 0.61111, 0, 0, 0.525],
    52: [0, 0.61111, 0, 0, 0.525],
    53: [0, 0.61111, 0, 0, 0.525],
    54: [0, 0.61111, 0, 0, 0.525],
    55: [0, 0.61111, 0, 0, 0.525],
    56: [0, 0.61111, 0, 0, 0.525],
    57: [0, 0.61111, 0, 0, 0.525],
    58: [0, 0.43056, 0, 0, 0.525],
    59: [0.13889, 0.43056, 0, 0, 0.525],
    60: [-0.05556, 0.55556, 0, 0, 0.525],
    61: [-0.19549, 0.41562, 0, 0, 0.525],
    62: [-0.05556, 0.55556, 0, 0, 0.525],
    63: [0, 0.61111, 0, 0, 0.525],
    64: [0, 0.61111, 0, 0, 0.525],
    65: [0, 0.61111, 0, 0, 0.525],
    66: [0, 0.61111, 0, 0, 0.525],
    67: [0, 0.61111, 0, 0, 0.525],
    68: [0, 0.61111, 0, 0, 0.525],
    69: [0, 0.61111, 0, 0, 0.525],
    70: [0, 0.61111, 0, 0, 0.525],
    71: [0, 0.61111, 0, 0, 0.525],
    72: [0, 0.61111, 0, 0, 0.525],
    73: [0, 0.61111, 0, 0, 0.525],
    74: [0, 0.61111, 0, 0, 0.525],
    75: [0, 0.61111, 0, 0, 0.525],
    76: [0, 0.61111, 0, 0, 0.525],
    77: [0, 0.61111, 0, 0, 0.525],
    78: [0, 0.61111, 0, 0, 0.525],
    79: [0, 0.61111, 0, 0, 0.525],
    80: [0, 0.61111, 0, 0, 0.525],
    81: [0.13889, 0.61111, 0, 0, 0.525],
    82: [0, 0.61111, 0, 0, 0.525],
    83: [0, 0.61111, 0, 0, 0.525],
    84: [0, 0.61111, 0, 0, 0.525],
    85: [0, 0.61111, 0, 0, 0.525],
    86: [0, 0.61111, 0, 0, 0.525],
    87: [0, 0.61111, 0, 0, 0.525],
    88: [0, 0.61111, 0, 0, 0.525],
    89: [0, 0.61111, 0, 0, 0.525],
    90: [0, 0.61111, 0, 0, 0.525],
    91: [0.08333, 0.69444, 0, 0, 0.525],
    92: [0.08333, 0.69444, 0, 0, 0.525],
    93: [0.08333, 0.69444, 0, 0, 0.525],
    94: [0, 0.61111, 0, 0, 0.525],
    95: [0.09514, 0, 0, 0, 0.525],
    96: [0, 0.61111, 0, 0, 0.525],
    97: [0, 0.43056, 0, 0, 0.525],
    98: [0, 0.61111, 0, 0, 0.525],
    99: [0, 0.43056, 0, 0, 0.525],
    100: [0, 0.61111, 0, 0, 0.525],
    101: [0, 0.43056, 0, 0, 0.525],
    102: [0, 0.61111, 0, 0, 0.525],
    103: [0.22222, 0.43056, 0, 0, 0.525],
    104: [0, 0.61111, 0, 0, 0.525],
    105: [0, 0.61111, 0, 0, 0.525],
    106: [0.22222, 0.61111, 0, 0, 0.525],
    107: [0, 0.61111, 0, 0, 0.525],
    108: [0, 0.61111, 0, 0, 0.525],
    109: [0, 0.43056, 0, 0, 0.525],
    110: [0, 0.43056, 0, 0, 0.525],
    111: [0, 0.43056, 0, 0, 0.525],
    112: [0.22222, 0.43056, 0, 0, 0.525],
    113: [0.22222, 0.43056, 0, 0, 0.525],
    114: [0, 0.43056, 0, 0, 0.525],
    115: [0, 0.43056, 0, 0, 0.525],
    116: [0, 0.55358, 0, 0, 0.525],
    117: [0, 0.43056, 0, 0, 0.525],
    118: [0, 0.43056, 0, 0, 0.525],
    119: [0, 0.43056, 0, 0, 0.525],
    120: [0, 0.43056, 0, 0, 0.525],
    121: [0.22222, 0.43056, 0, 0, 0.525],
    122: [0, 0.43056, 0, 0, 0.525],
    123: [0.08333, 0.69444, 0, 0, 0.525],
    124: [0.08333, 0.69444, 0, 0, 0.525],
    125: [0.08333, 0.69444, 0, 0, 0.525],
    126: [0, 0.61111, 0, 0, 0.525],
    127: [0, 0.61111, 0, 0, 0.525],
    160: [0, 0, 0, 0, 0.525],
    176: [0, 0.61111, 0, 0, 0.525],
    184: [0.19445, 0, 0, 0, 0.525],
    305: [0, 0.43056, 0, 0, 0.525],
    567: [0.22222, 0.43056, 0, 0, 0.525],
    711: [0, 0.56597, 0, 0, 0.525],
    713: [0, 0.56555, 0, 0, 0.525],
    714: [0, 0.61111, 0, 0, 0.525],
    715: [0, 0.61111, 0, 0, 0.525],
    728: [0, 0.61111, 0, 0, 0.525],
    730: [0, 0.61111, 0, 0, 0.525],
    770: [0, 0.61111, 0, 0, 0.525],
    771: [0, 0.61111, 0, 0, 0.525],
    776: [0, 0.61111, 0, 0, 0.525],
    915: [0, 0.61111, 0, 0, 0.525],
    916: [0, 0.61111, 0, 0, 0.525],
    920: [0, 0.61111, 0, 0, 0.525],
    923: [0, 0.61111, 0, 0, 0.525],
    926: [0, 0.61111, 0, 0, 0.525],
    928: [0, 0.61111, 0, 0, 0.525],
    931: [0, 0.61111, 0, 0, 0.525],
    933: [0, 0.61111, 0, 0, 0.525],
    934: [0, 0.61111, 0, 0, 0.525],
    936: [0, 0.61111, 0, 0, 0.525],
    937: [0, 0.61111, 0, 0, 0.525],
    8216: [0, 0.61111, 0, 0, 0.525],
    8217: [0, 0.61111, 0, 0, 0.525],
    8242: [0, 0.61111, 0, 0, 0.525],
    9251: [0.11111, 0.21944, 0, 0, 0.525]
  }
}, Be = {
  slant: [0.25, 0.25, 0.25],
  // sigma1
  space: [0, 0, 0],
  // sigma2
  stretch: [0, 0, 0],
  // sigma3
  shrink: [0, 0, 0],
  // sigma4
  xHeight: [0.431, 0.431, 0.431],
  // sigma5
  quad: [1, 1.171, 1.472],
  // sigma6
  extraSpace: [0, 0, 0],
  // sigma7
  num1: [0.677, 0.732, 0.925],
  // sigma8
  num2: [0.394, 0.384, 0.387],
  // sigma9
  num3: [0.444, 0.471, 0.504],
  // sigma10
  denom1: [0.686, 0.752, 1.025],
  // sigma11
  denom2: [0.345, 0.344, 0.532],
  // sigma12
  sup1: [0.413, 0.503, 0.504],
  // sigma13
  sup2: [0.363, 0.431, 0.404],
  // sigma14
  sup3: [0.289, 0.286, 0.294],
  // sigma15
  sub1: [0.15, 0.143, 0.2],
  // sigma16
  sub2: [0.247, 0.286, 0.4],
  // sigma17
  supDrop: [0.386, 0.353, 0.494],
  // sigma18
  subDrop: [0.05, 0.071, 0.1],
  // sigma19
  delim1: [2.39, 1.7, 1.98],
  // sigma20
  delim2: [1.01, 1.157, 1.42],
  // sigma21
  axisHeight: [0.25, 0.25, 0.25],
  // sigma22
  // These font metrics are extracted from TeX by using tftopl on cmex10.tfm;
  // they correspond to the font parameters of the extension fonts (family 3).
  // See the TeXbook, page 441. In AMSTeX, the extension fonts scale; to
  // match cmex7, we'd use cmex7.tfm values for script and scriptscript
  // values.
  defaultRuleThickness: [0.04, 0.049, 0.049],
  // xi8; cmex7: 0.049
  bigOpSpacing1: [0.111, 0.111, 0.111],
  // xi9
  bigOpSpacing2: [0.166, 0.166, 0.166],
  // xi10
  bigOpSpacing3: [0.2, 0.2, 0.2],
  // xi11
  bigOpSpacing4: [0.6, 0.611, 0.611],
  // xi12; cmex7: 0.611
  bigOpSpacing5: [0.1, 0.143, 0.143],
  // xi13; cmex7: 0.143
  // The \sqrt rule width is taken from the height of the surd character.
  // Since we use the same font at all sizes, this thickness doesn't scale.
  sqrtRuleThickness: [0.04, 0.04, 0.04],
  // This value determines how large a pt is, for metrics which are defined
  // in terms of pts.
  // This value is also used in katex.scss; if you change it make sure the
  // values match.
  ptPerEm: [10, 10, 10],
  // The space between adjacent `|` columns in an array definition. From
  // `\showthe\doublerulesep` in LaTeX. Equals 2.0 / ptPerEm.
  doubleRuleSep: [0.2, 0.2, 0.2],
  // The width of separator lines in {array} environments. From
  // `\showthe\arrayrulewidth` in LaTeX. Equals 0.4 / ptPerEm.
  arrayRuleWidth: [0.04, 0.04, 0.04],
  // Two values from LaTeX source2e:
  fboxsep: [0.3, 0.3, 0.3],
  //        3 pt / ptPerEm
  fboxrule: [0.04, 0.04, 0.04]
  // 0.4 pt / ptPerEm
}, cr = {
  // Latin-1
  Å: "A",
  Ð: "D",
  Þ: "o",
  å: "a",
  ð: "d",
  þ: "o",
  // Cyrillic
  А: "A",
  Б: "B",
  В: "B",
  Г: "F",
  Д: "A",
  Е: "E",
  Ж: "K",
  З: "3",
  И: "N",
  Й: "N",
  К: "K",
  Л: "N",
  М: "M",
  Н: "H",
  О: "O",
  П: "N",
  Р: "P",
  С: "C",
  Т: "T",
  У: "y",
  Ф: "O",
  Х: "X",
  Ц: "U",
  Ч: "h",
  Ш: "W",
  Щ: "W",
  Ъ: "B",
  Ы: "X",
  Ь: "B",
  Э: "3",
  Ю: "X",
  Я: "R",
  а: "a",
  б: "b",
  в: "a",
  г: "r",
  д: "y",
  е: "e",
  ж: "m",
  з: "e",
  и: "n",
  й: "n",
  к: "n",
  л: "n",
  м: "m",
  н: "n",
  о: "o",
  п: "n",
  р: "p",
  с: "c",
  т: "o",
  у: "y",
  ф: "b",
  х: "x",
  ц: "n",
  ч: "n",
  ш: "w",
  щ: "w",
  ъ: "a",
  ы: "m",
  ь: "a",
  э: "e",
  ю: "m",
  я: "r"
};
function Zn(r, e) {
  I0[r] = e;
}
function Xt(r, e, t) {
  if (!I0[e])
    throw new Error("Font metrics not found for font: " + e + ".");
  var a = r.charCodeAt(0), n = I0[e][a];
  if (!n && r[0] in cr && (a = cr[r[0]].charCodeAt(0), n = I0[e][a]), !n && t === "text" && Zr(a) && (n = I0[e][77]), n)
    return {
      depth: n[0],
      height: n[1],
      italic: n[2],
      skew: n[3],
      width: n[4]
    };
}
var it = {};
function Kn(r) {
  var e;
  if (r >= 5 ? e = 0 : r >= 3 ? e = 1 : e = 2, !it[e]) {
    var t = it[e] = {
      cssEmPerMu: Be.quad[e] / 18
    };
    for (var a in Be)
      Be.hasOwnProperty(a) && (t[a] = Be[a][e]);
  }
  return it[e];
}
var Q = {
  math: {},
  text: {}
};
function i(r, e, t, a, n, s) {
  Q[r][n] = {
    font: e,
    group: t,
    replace: a
  }, s && a && (Q[r][a] = Q[r][n]);
}
var l = "math", M = "text", c = "main", f = "ams", e0 = "accent-token", L = "bin", g0 = "close", ye = "inner", P = "mathord", l0 = "op-token", M0 = "open", Me = "punct", p = "rel", j0 = "spacing", g = "textord";
i(l, c, p, "≡", "\\equiv", !0);
i(l, c, p, "≺", "\\prec", !0);
i(l, c, p, "≻", "\\succ", !0);
i(l, c, p, "∼", "\\sim", !0);
i(l, c, p, "⊥", "\\perp");
i(l, c, p, "⪯", "\\preceq", !0);
i(l, c, p, "⪰", "\\succeq", !0);
i(l, c, p, "≃", "\\simeq", !0);
i(l, c, p, "∣", "\\mid", !0);
i(l, c, p, "≪", "\\ll", !0);
i(l, c, p, "≫", "\\gg", !0);
i(l, c, p, "≍", "\\asymp", !0);
i(l, c, p, "∥", "\\parallel");
i(l, c, p, "⋈", "\\bowtie", !0);
i(l, c, p, "⌣", "\\smile", !0);
i(l, c, p, "⊑", "\\sqsubseteq", !0);
i(l, c, p, "⊒", "\\sqsupseteq", !0);
i(l, c, p, "≐", "\\doteq", !0);
i(l, c, p, "⌢", "\\frown", !0);
i(l, c, p, "∋", "\\ni", !0);
i(l, c, p, "∝", "\\propto", !0);
i(l, c, p, "⊢", "\\vdash", !0);
i(l, c, p, "⊣", "\\dashv", !0);
i(l, c, p, "∋", "\\owns");
i(l, c, Me, ".", "\\ldotp");
i(l, c, Me, "⋅", "\\cdotp");
i(l, c, Me, "⋅", "·");
i(M, c, g, "⋅", "·");
i(l, c, g, "#", "\\#");
i(M, c, g, "#", "\\#");
i(l, c, g, "&", "\\&");
i(M, c, g, "&", "\\&");
i(l, c, g, "ℵ", "\\aleph", !0);
i(l, c, g, "∀", "\\forall", !0);
i(l, c, g, "ℏ", "\\hbar", !0);
i(l, c, g, "∃", "\\exists", !0);
i(l, c, g, "∇", "\\nabla", !0);
i(l, c, g, "♭", "\\flat", !0);
i(l, c, g, "ℓ", "\\ell", !0);
i(l, c, g, "♮", "\\natural", !0);
i(l, c, g, "♣", "\\clubsuit", !0);
i(l, c, g, "℘", "\\wp", !0);
i(l, c, g, "♯", "\\sharp", !0);
i(l, c, g, "♢", "\\diamondsuit", !0);
i(l, c, g, "ℜ", "\\Re", !0);
i(l, c, g, "♡", "\\heartsuit", !0);
i(l, c, g, "ℑ", "\\Im", !0);
i(l, c, g, "♠", "\\spadesuit", !0);
i(l, c, g, "§", "\\S", !0);
i(M, c, g, "§", "\\S");
i(l, c, g, "¶", "\\P", !0);
i(M, c, g, "¶", "\\P");
i(l, c, g, "†", "\\dag");
i(M, c, g, "†", "\\dag");
i(M, c, g, "†", "\\textdagger");
i(l, c, g, "‡", "\\ddag");
i(M, c, g, "‡", "\\ddag");
i(M, c, g, "‡", "\\textdaggerdbl");
i(l, c, g0, "⎱", "\\rmoustache", !0);
i(l, c, M0, "⎰", "\\lmoustache", !0);
i(l, c, g0, "⟯", "\\rgroup", !0);
i(l, c, M0, "⟮", "\\lgroup", !0);
i(l, c, L, "∓", "\\mp", !0);
i(l, c, L, "⊖", "\\ominus", !0);
i(l, c, L, "⊎", "\\uplus", !0);
i(l, c, L, "⊓", "\\sqcap", !0);
i(l, c, L, "∗", "\\ast");
i(l, c, L, "⊔", "\\sqcup", !0);
i(l, c, L, "◯", "\\bigcirc", !0);
i(l, c, L, "∙", "\\bullet", !0);
i(l, c, L, "‡", "\\ddagger");
i(l, c, L, "≀", "\\wr", !0);
i(l, c, L, "⨿", "\\amalg");
i(l, c, L, "&", "\\And");
i(l, c, p, "⟵", "\\longleftarrow", !0);
i(l, c, p, "⇐", "\\Leftarrow", !0);
i(l, c, p, "⟸", "\\Longleftarrow", !0);
i(l, c, p, "⟶", "\\longrightarrow", !0);
i(l, c, p, "⇒", "\\Rightarrow", !0);
i(l, c, p, "⟹", "\\Longrightarrow", !0);
i(l, c, p, "↔", "\\leftrightarrow", !0);
i(l, c, p, "⟷", "\\longleftrightarrow", !0);
i(l, c, p, "⇔", "\\Leftrightarrow", !0);
i(l, c, p, "⟺", "\\Longleftrightarrow", !0);
i(l, c, p, "↦", "\\mapsto", !0);
i(l, c, p, "⟼", "\\longmapsto", !0);
i(l, c, p, "↗", "\\nearrow", !0);
i(l, c, p, "↩", "\\hookleftarrow", !0);
i(l, c, p, "↪", "\\hookrightarrow", !0);
i(l, c, p, "↘", "\\searrow", !0);
i(l, c, p, "↼", "\\leftharpoonup", !0);
i(l, c, p, "⇀", "\\rightharpoonup", !0);
i(l, c, p, "↙", "\\swarrow", !0);
i(l, c, p, "↽", "\\leftharpoondown", !0);
i(l, c, p, "⇁", "\\rightharpoondown", !0);
i(l, c, p, "↖", "\\nwarrow", !0);
i(l, c, p, "⇌", "\\rightleftharpoons", !0);
i(l, f, p, "≮", "\\nless", !0);
i(l, f, p, "", "\\@nleqslant");
i(l, f, p, "", "\\@nleqq");
i(l, f, p, "⪇", "\\lneq", !0);
i(l, f, p, "≨", "\\lneqq", !0);
i(l, f, p, "", "\\@lvertneqq");
i(l, f, p, "⋦", "\\lnsim", !0);
i(l, f, p, "⪉", "\\lnapprox", !0);
i(l, f, p, "⊀", "\\nprec", !0);
i(l, f, p, "⋠", "\\npreceq", !0);
i(l, f, p, "⋨", "\\precnsim", !0);
i(l, f, p, "⪹", "\\precnapprox", !0);
i(l, f, p, "≁", "\\nsim", !0);
i(l, f, p, "", "\\@nshortmid");
i(l, f, p, "∤", "\\nmid", !0);
i(l, f, p, "⊬", "\\nvdash", !0);
i(l, f, p, "⊭", "\\nvDash", !0);
i(l, f, p, "⋪", "\\ntriangleleft");
i(l, f, p, "⋬", "\\ntrianglelefteq", !0);
i(l, f, p, "⊊", "\\subsetneq", !0);
i(l, f, p, "", "\\@varsubsetneq");
i(l, f, p, "⫋", "\\subsetneqq", !0);
i(l, f, p, "", "\\@varsubsetneqq");
i(l, f, p, "≯", "\\ngtr", !0);
i(l, f, p, "", "\\@ngeqslant");
i(l, f, p, "", "\\@ngeqq");
i(l, f, p, "⪈", "\\gneq", !0);
i(l, f, p, "≩", "\\gneqq", !0);
i(l, f, p, "", "\\@gvertneqq");
i(l, f, p, "⋧", "\\gnsim", !0);
i(l, f, p, "⪊", "\\gnapprox", !0);
i(l, f, p, "⊁", "\\nsucc", !0);
i(l, f, p, "⋡", "\\nsucceq", !0);
i(l, f, p, "⋩", "\\succnsim", !0);
i(l, f, p, "⪺", "\\succnapprox", !0);
i(l, f, p, "≆", "\\ncong", !0);
i(l, f, p, "", "\\@nshortparallel");
i(l, f, p, "∦", "\\nparallel", !0);
i(l, f, p, "⊯", "\\nVDash", !0);
i(l, f, p, "⋫", "\\ntriangleright");
i(l, f, p, "⋭", "\\ntrianglerighteq", !0);
i(l, f, p, "", "\\@nsupseteqq");
i(l, f, p, "⊋", "\\supsetneq", !0);
i(l, f, p, "", "\\@varsupsetneq");
i(l, f, p, "⫌", "\\supsetneqq", !0);
i(l, f, p, "", "\\@varsupsetneqq");
i(l, f, p, "⊮", "\\nVdash", !0);
i(l, f, p, "⪵", "\\precneqq", !0);
i(l, f, p, "⪶", "\\succneqq", !0);
i(l, f, p, "", "\\@nsubseteqq");
i(l, f, L, "⊴", "\\unlhd");
i(l, f, L, "⊵", "\\unrhd");
i(l, f, p, "↚", "\\nleftarrow", !0);
i(l, f, p, "↛", "\\nrightarrow", !0);
i(l, f, p, "⇍", "\\nLeftarrow", !0);
i(l, f, p, "⇏", "\\nRightarrow", !0);
i(l, f, p, "↮", "\\nleftrightarrow", !0);
i(l, f, p, "⇎", "\\nLeftrightarrow", !0);
i(l, f, p, "△", "\\vartriangle");
i(l, f, g, "ℏ", "\\hslash");
i(l, f, g, "▽", "\\triangledown");
i(l, f, g, "◊", "\\lozenge");
i(l, f, g, "Ⓢ", "\\circledS");
i(l, f, g, "®", "\\circledR");
i(M, f, g, "®", "\\circledR");
i(l, f, g, "∡", "\\measuredangle", !0);
i(l, f, g, "∄", "\\nexists");
i(l, f, g, "℧", "\\mho");
i(l, f, g, "Ⅎ", "\\Finv", !0);
i(l, f, g, "⅁", "\\Game", !0);
i(l, f, g, "‵", "\\backprime");
i(l, f, g, "▲", "\\blacktriangle");
i(l, f, g, "▼", "\\blacktriangledown");
i(l, f, g, "■", "\\blacksquare");
i(l, f, g, "⧫", "\\blacklozenge");
i(l, f, g, "★", "\\bigstar");
i(l, f, g, "∢", "\\sphericalangle", !0);
i(l, f, g, "∁", "\\complement", !0);
i(l, f, g, "ð", "\\eth", !0);
i(M, c, g, "ð", "ð");
i(l, f, g, "╱", "\\diagup");
i(l, f, g, "╲", "\\diagdown");
i(l, f, g, "□", "\\square");
i(l, f, g, "□", "\\Box");
i(l, f, g, "◊", "\\Diamond");
i(l, f, g, "¥", "\\yen", !0);
i(M, f, g, "¥", "\\yen", !0);
i(l, f, g, "✓", "\\checkmark", !0);
i(M, f, g, "✓", "\\checkmark");
i(l, f, g, "ℶ", "\\beth", !0);
i(l, f, g, "ℸ", "\\daleth", !0);
i(l, f, g, "ℷ", "\\gimel", !0);
i(l, f, g, "ϝ", "\\digamma", !0);
i(l, f, g, "ϰ", "\\varkappa");
i(l, f, M0, "┌", "\\@ulcorner", !0);
i(l, f, g0, "┐", "\\@urcorner", !0);
i(l, f, M0, "└", "\\@llcorner", !0);
i(l, f, g0, "┘", "\\@lrcorner", !0);
i(l, f, p, "≦", "\\leqq", !0);
i(l, f, p, "⩽", "\\leqslant", !0);
i(l, f, p, "⪕", "\\eqslantless", !0);
i(l, f, p, "≲", "\\lesssim", !0);
i(l, f, p, "⪅", "\\lessapprox", !0);
i(l, f, p, "≊", "\\approxeq", !0);
i(l, f, L, "⋖", "\\lessdot");
i(l, f, p, "⋘", "\\lll", !0);
i(l, f, p, "≶", "\\lessgtr", !0);
i(l, f, p, "⋚", "\\lesseqgtr", !0);
i(l, f, p, "⪋", "\\lesseqqgtr", !0);
i(l, f, p, "≑", "\\doteqdot");
i(l, f, p, "≓", "\\risingdotseq", !0);
i(l, f, p, "≒", "\\fallingdotseq", !0);
i(l, f, p, "∽", "\\backsim", !0);
i(l, f, p, "⋍", "\\backsimeq", !0);
i(l, f, p, "⫅", "\\subseteqq", !0);
i(l, f, p, "⋐", "\\Subset", !0);
i(l, f, p, "⊏", "\\sqsubset", !0);
i(l, f, p, "≼", "\\preccurlyeq", !0);
i(l, f, p, "⋞", "\\curlyeqprec", !0);
i(l, f, p, "≾", "\\precsim", !0);
i(l, f, p, "⪷", "\\precapprox", !0);
i(l, f, p, "⊲", "\\vartriangleleft");
i(l, f, p, "⊴", "\\trianglelefteq");
i(l, f, p, "⊨", "\\vDash", !0);
i(l, f, p, "⊪", "\\Vvdash", !0);
i(l, f, p, "⌣", "\\smallsmile");
i(l, f, p, "⌢", "\\smallfrown");
i(l, f, p, "≏", "\\bumpeq", !0);
i(l, f, p, "≎", "\\Bumpeq", !0);
i(l, f, p, "≧", "\\geqq", !0);
i(l, f, p, "⩾", "\\geqslant", !0);
i(l, f, p, "⪖", "\\eqslantgtr", !0);
i(l, f, p, "≳", "\\gtrsim", !0);
i(l, f, p, "⪆", "\\gtrapprox", !0);
i(l, f, L, "⋗", "\\gtrdot");
i(l, f, p, "⋙", "\\ggg", !0);
i(l, f, p, "≷", "\\gtrless", !0);
i(l, f, p, "⋛", "\\gtreqless", !0);
i(l, f, p, "⪌", "\\gtreqqless", !0);
i(l, f, p, "≖", "\\eqcirc", !0);
i(l, f, p, "≗", "\\circeq", !0);
i(l, f, p, "≜", "\\triangleq", !0);
i(l, f, p, "∼", "\\thicksim");
i(l, f, p, "≈", "\\thickapprox");
i(l, f, p, "⫆", "\\supseteqq", !0);
i(l, f, p, "⋑", "\\Supset", !0);
i(l, f, p, "⊐", "\\sqsupset", !0);
i(l, f, p, "≽", "\\succcurlyeq", !0);
i(l, f, p, "⋟", "\\curlyeqsucc", !0);
i(l, f, p, "≿", "\\succsim", !0);
i(l, f, p, "⪸", "\\succapprox", !0);
i(l, f, p, "⊳", "\\vartriangleright");
i(l, f, p, "⊵", "\\trianglerighteq");
i(l, f, p, "⊩", "\\Vdash", !0);
i(l, f, p, "∣", "\\shortmid");
i(l, f, p, "∥", "\\shortparallel");
i(l, f, p, "≬", "\\between", !0);
i(l, f, p, "⋔", "\\pitchfork", !0);
i(l, f, p, "∝", "\\varpropto");
i(l, f, p, "◀", "\\blacktriangleleft");
i(l, f, p, "∴", "\\therefore", !0);
i(l, f, p, "∍", "\\backepsilon");
i(l, f, p, "▶", "\\blacktriangleright");
i(l, f, p, "∵", "\\because", !0);
i(l, f, p, "⋘", "\\llless");
i(l, f, p, "⋙", "\\gggtr");
i(l, f, L, "⊲", "\\lhd");
i(l, f, L, "⊳", "\\rhd");
i(l, f, p, "≂", "\\eqsim", !0);
i(l, c, p, "⋈", "\\Join");
i(l, f, p, "≑", "\\Doteq", !0);
i(l, f, L, "∔", "\\dotplus", !0);
i(l, f, L, "∖", "\\smallsetminus");
i(l, f, L, "⋒", "\\Cap", !0);
i(l, f, L, "⋓", "\\Cup", !0);
i(l, f, L, "⩞", "\\doublebarwedge", !0);
i(l, f, L, "⊟", "\\boxminus", !0);
i(l, f, L, "⊞", "\\boxplus", !0);
i(l, f, L, "⋇", "\\divideontimes", !0);
i(l, f, L, "⋉", "\\ltimes", !0);
i(l, f, L, "⋊", "\\rtimes", !0);
i(l, f, L, "⋋", "\\leftthreetimes", !0);
i(l, f, L, "⋌", "\\rightthreetimes", !0);
i(l, f, L, "⋏", "\\curlywedge", !0);
i(l, f, L, "⋎", "\\curlyvee", !0);
i(l, f, L, "⊝", "\\circleddash", !0);
i(l, f, L, "⊛", "\\circledast", !0);
i(l, f, L, "⋅", "\\centerdot");
i(l, f, L, "⊺", "\\intercal", !0);
i(l, f, L, "⋒", "\\doublecap");
i(l, f, L, "⋓", "\\doublecup");
i(l, f, L, "⊠", "\\boxtimes", !0);
i(l, f, p, "⇢", "\\dashrightarrow", !0);
i(l, f, p, "⇠", "\\dashleftarrow", !0);
i(l, f, p, "⇇", "\\leftleftarrows", !0);
i(l, f, p, "⇆", "\\leftrightarrows", !0);
i(l, f, p, "⇚", "\\Lleftarrow", !0);
i(l, f, p, "↞", "\\twoheadleftarrow", !0);
i(l, f, p, "↢", "\\leftarrowtail", !0);
i(l, f, p, "↫", "\\looparrowleft", !0);
i(l, f, p, "⇋", "\\leftrightharpoons", !0);
i(l, f, p, "↶", "\\curvearrowleft", !0);
i(l, f, p, "↺", "\\circlearrowleft", !0);
i(l, f, p, "↰", "\\Lsh", !0);
i(l, f, p, "⇈", "\\upuparrows", !0);
i(l, f, p, "↿", "\\upharpoonleft", !0);
i(l, f, p, "⇃", "\\downharpoonleft", !0);
i(l, c, p, "⊶", "\\origof", !0);
i(l, c, p, "⊷", "\\imageof", !0);
i(l, f, p, "⊸", "\\multimap", !0);
i(l, f, p, "↭", "\\leftrightsquigarrow", !0);
i(l, f, p, "⇉", "\\rightrightarrows", !0);
i(l, f, p, "⇄", "\\rightleftarrows", !0);
i(l, f, p, "↠", "\\twoheadrightarrow", !0);
i(l, f, p, "↣", "\\rightarrowtail", !0);
i(l, f, p, "↬", "\\looparrowright", !0);
i(l, f, p, "↷", "\\curvearrowright", !0);
i(l, f, p, "↻", "\\circlearrowright", !0);
i(l, f, p, "↱", "\\Rsh", !0);
i(l, f, p, "⇊", "\\downdownarrows", !0);
i(l, f, p, "↾", "\\upharpoonright", !0);
i(l, f, p, "⇂", "\\downharpoonright", !0);
i(l, f, p, "⇝", "\\rightsquigarrow", !0);
i(l, f, p, "⇝", "\\leadsto");
i(l, f, p, "⇛", "\\Rrightarrow", !0);
i(l, f, p, "↾", "\\restriction");
i(l, c, g, "‘", "`");
i(l, c, g, "$", "\\$");
i(M, c, g, "$", "\\$");
i(M, c, g, "$", "\\textdollar");
i(l, c, g, "%", "\\%");
i(M, c, g, "%", "\\%");
i(l, c, g, "_", "\\_");
i(M, c, g, "_", "\\_");
i(M, c, g, "_", "\\textunderscore");
i(l, c, g, "∠", "\\angle", !0);
i(l, c, g, "∞", "\\infty", !0);
i(l, c, g, "′", "\\prime");
i(l, c, g, "△", "\\triangle");
i(l, c, g, "Γ", "\\Gamma", !0);
i(l, c, g, "Δ", "\\Delta", !0);
i(l, c, g, "Θ", "\\Theta", !0);
i(l, c, g, "Λ", "\\Lambda", !0);
i(l, c, g, "Ξ", "\\Xi", !0);
i(l, c, g, "Π", "\\Pi", !0);
i(l, c, g, "Σ", "\\Sigma", !0);
i(l, c, g, "Υ", "\\Upsilon", !0);
i(l, c, g, "Φ", "\\Phi", !0);
i(l, c, g, "Ψ", "\\Psi", !0);
i(l, c, g, "Ω", "\\Omega", !0);
i(l, c, g, "A", "Α");
i(l, c, g, "B", "Β");
i(l, c, g, "E", "Ε");
i(l, c, g, "Z", "Ζ");
i(l, c, g, "H", "Η");
i(l, c, g, "I", "Ι");
i(l, c, g, "K", "Κ");
i(l, c, g, "M", "Μ");
i(l, c, g, "N", "Ν");
i(l, c, g, "O", "Ο");
i(l, c, g, "P", "Ρ");
i(l, c, g, "T", "Τ");
i(l, c, g, "X", "Χ");
i(l, c, g, "¬", "\\neg", !0);
i(l, c, g, "¬", "\\lnot");
i(l, c, g, "⊤", "\\top");
i(l, c, g, "⊥", "\\bot");
i(l, c, g, "∅", "\\emptyset");
i(l, f, g, "∅", "\\varnothing");
i(l, c, P, "α", "\\alpha", !0);
i(l, c, P, "β", "\\beta", !0);
i(l, c, P, "γ", "\\gamma", !0);
i(l, c, P, "δ", "\\delta", !0);
i(l, c, P, "ϵ", "\\epsilon", !0);
i(l, c, P, "ζ", "\\zeta", !0);
i(l, c, P, "η", "\\eta", !0);
i(l, c, P, "θ", "\\theta", !0);
i(l, c, P, "ι", "\\iota", !0);
i(l, c, P, "κ", "\\kappa", !0);
i(l, c, P, "λ", "\\lambda", !0);
i(l, c, P, "μ", "\\mu", !0);
i(l, c, P, "ν", "\\nu", !0);
i(l, c, P, "ξ", "\\xi", !0);
i(l, c, P, "ο", "\\omicron", !0);
i(l, c, P, "π", "\\pi", !0);
i(l, c, P, "ρ", "\\rho", !0);
i(l, c, P, "σ", "\\sigma", !0);
i(l, c, P, "τ", "\\tau", !0);
i(l, c, P, "υ", "\\upsilon", !0);
i(l, c, P, "ϕ", "\\phi", !0);
i(l, c, P, "χ", "\\chi", !0);
i(l, c, P, "ψ", "\\psi", !0);
i(l, c, P, "ω", "\\omega", !0);
i(l, c, P, "ε", "\\varepsilon", !0);
i(l, c, P, "ϑ", "\\vartheta", !0);
i(l, c, P, "ϖ", "\\varpi", !0);
i(l, c, P, "ϱ", "\\varrho", !0);
i(l, c, P, "ς", "\\varsigma", !0);
i(l, c, P, "φ", "\\varphi", !0);
i(l, c, L, "∗", "*", !0);
i(l, c, L, "+", "+");
i(l, c, L, "−", "-", !0);
i(l, c, L, "⋅", "\\cdot", !0);
i(l, c, L, "∘", "\\circ", !0);
i(l, c, L, "÷", "\\div", !0);
i(l, c, L, "±", "\\pm", !0);
i(l, c, L, "×", "\\times", !0);
i(l, c, L, "∩", "\\cap", !0);
i(l, c, L, "∪", "\\cup", !0);
i(l, c, L, "∖", "\\setminus", !0);
i(l, c, L, "∧", "\\land");
i(l, c, L, "∨", "\\lor");
i(l, c, L, "∧", "\\wedge", !0);
i(l, c, L, "∨", "\\vee", !0);
i(l, c, g, "√", "\\surd");
i(l, c, M0, "⟨", "\\langle", !0);
i(l, c, M0, "∣", "\\lvert");
i(l, c, M0, "∥", "\\lVert");
i(l, c, g0, "?", "?");
i(l, c, g0, "!", "!");
i(l, c, g0, "⟩", "\\rangle", !0);
i(l, c, g0, "∣", "\\rvert");
i(l, c, g0, "∥", "\\rVert");
i(l, c, p, "=", "=");
i(l, c, p, ":", ":");
i(l, c, p, "≈", "\\approx", !0);
i(l, c, p, "≅", "\\cong", !0);
i(l, c, p, "≥", "\\ge");
i(l, c, p, "≥", "\\geq", !0);
i(l, c, p, "←", "\\gets");
i(l, c, p, ">", "\\gt", !0);
i(l, c, p, "∈", "\\in", !0);
i(l, c, p, "", "\\@not");
i(l, c, p, "⊂", "\\subset", !0);
i(l, c, p, "⊃", "\\supset", !0);
i(l, c, p, "⊆", "\\subseteq", !0);
i(l, c, p, "⊇", "\\supseteq", !0);
i(l, f, p, "⊈", "\\nsubseteq", !0);
i(l, f, p, "⊉", "\\nsupseteq", !0);
i(l, c, p, "⊨", "\\models");
i(l, c, p, "←", "\\leftarrow", !0);
i(l, c, p, "≤", "\\le");
i(l, c, p, "≤", "\\leq", !0);
i(l, c, p, "<", "\\lt", !0);
i(l, c, p, "→", "\\rightarrow", !0);
i(l, c, p, "→", "\\to");
i(l, f, p, "≱", "\\ngeq", !0);
i(l, f, p, "≰", "\\nleq", !0);
i(l, c, j0, " ", "\\ ");
i(l, c, j0, " ", "\\space");
i(l, c, j0, " ", "\\nobreakspace");
i(M, c, j0, " ", "\\ ");
i(M, c, j0, " ", " ");
i(M, c, j0, " ", "\\space");
i(M, c, j0, " ", "\\nobreakspace");
i(l, c, j0, "", "\\nobreak");
i(l, c, j0, "", "\\allowbreak");
i(l, c, Me, ",", ",");
i(l, c, Me, ";", ";");
i(l, f, L, "⊼", "\\barwedge", !0);
i(l, f, L, "⊻", "\\veebar", !0);
i(l, c, L, "⊙", "\\odot", !0);
i(l, c, L, "⊕", "\\oplus", !0);
i(l, c, L, "⊗", "\\otimes", !0);
i(l, c, g, "∂", "\\partial", !0);
i(l, c, L, "⊘", "\\oslash", !0);
i(l, f, L, "⊚", "\\circledcirc", !0);
i(l, f, L, "⊡", "\\boxdot", !0);
i(l, c, L, "△", "\\bigtriangleup");
i(l, c, L, "▽", "\\bigtriangledown");
i(l, c, L, "†", "\\dagger");
i(l, c, L, "⋄", "\\diamond");
i(l, c, L, "⋆", "\\star");
i(l, c, L, "◃", "\\triangleleft");
i(l, c, L, "▹", "\\triangleright");
i(l, c, M0, "{", "\\{");
i(M, c, g, "{", "\\{");
i(M, c, g, "{", "\\textbraceleft");
i(l, c, g0, "}", "\\}");
i(M, c, g, "}", "\\}");
i(M, c, g, "}", "\\textbraceright");
i(l, c, M0, "{", "\\lbrace");
i(l, c, g0, "}", "\\rbrace");
i(l, c, M0, "[", "\\lbrack", !0);
i(M, c, g, "[", "\\lbrack", !0);
i(l, c, g0, "]", "\\rbrack", !0);
i(M, c, g, "]", "\\rbrack", !0);
i(l, c, M0, "(", "\\lparen", !0);
i(l, c, g0, ")", "\\rparen", !0);
i(M, c, g, "<", "\\textless", !0);
i(M, c, g, ">", "\\textgreater", !0);
i(l, c, M0, "⌊", "\\lfloor", !0);
i(l, c, g0, "⌋", "\\rfloor", !0);
i(l, c, M0, "⌈", "\\lceil", !0);
i(l, c, g0, "⌉", "\\rceil", !0);
i(l, c, g, "\\", "\\backslash");
i(l, c, g, "∣", "|");
i(l, c, g, "∣", "\\vert");
i(M, c, g, "|", "\\textbar", !0);
i(l, c, g, "∥", "\\|");
i(l, c, g, "∥", "\\Vert");
i(M, c, g, "∥", "\\textbardbl");
i(M, c, g, "~", "\\textasciitilde");
i(M, c, g, "\\", "\\textbackslash");
i(M, c, g, "^", "\\textasciicircum");
i(l, c, p, "↑", "\\uparrow", !0);
i(l, c, p, "⇑", "\\Uparrow", !0);
i(l, c, p, "↓", "\\downarrow", !0);
i(l, c, p, "⇓", "\\Downarrow", !0);
i(l, c, p, "↕", "\\updownarrow", !0);
i(l, c, p, "⇕", "\\Updownarrow", !0);
i(l, c, l0, "∐", "\\coprod");
i(l, c, l0, "⋁", "\\bigvee");
i(l, c, l0, "⋀", "\\bigwedge");
i(l, c, l0, "⨄", "\\biguplus");
i(l, c, l0, "⋂", "\\bigcap");
i(l, c, l0, "⋃", "\\bigcup");
i(l, c, l0, "∫", "\\int");
i(l, c, l0, "∫", "\\intop");
i(l, c, l0, "∬", "\\iint");
i(l, c, l0, "∭", "\\iiint");
i(l, c, l0, "∏", "\\prod");
i(l, c, l0, "∑", "\\sum");
i(l, c, l0, "⨂", "\\bigotimes");
i(l, c, l0, "⨁", "\\bigoplus");
i(l, c, l0, "⨀", "\\bigodot");
i(l, c, l0, "∮", "\\oint");
i(l, c, l0, "∯", "\\oiint");
i(l, c, l0, "∰", "\\oiiint");
i(l, c, l0, "⨆", "\\bigsqcup");
i(l, c, l0, "∫", "\\smallint");
i(M, c, ye, "…", "\\textellipsis");
i(l, c, ye, "…", "\\mathellipsis");
i(M, c, ye, "…", "\\ldots", !0);
i(l, c, ye, "…", "\\ldots", !0);
i(l, c, ye, "⋯", "\\@cdots", !0);
i(l, c, ye, "⋱", "\\ddots", !0);
i(l, c, g, "⋮", "\\varvdots");
i(M, c, g, "⋮", "\\varvdots");
i(l, c, e0, "ˊ", "\\acute");
i(l, c, e0, "ˋ", "\\grave");
i(l, c, e0, "¨", "\\ddot");
i(l, c, e0, "~", "\\tilde");
i(l, c, e0, "ˉ", "\\bar");
i(l, c, e0, "˘", "\\breve");
i(l, c, e0, "ˇ", "\\check");
i(l, c, e0, "^", "\\hat");
i(l, c, e0, "⃗", "\\vec");
i(l, c, e0, "˙", "\\dot");
i(l, c, e0, "˚", "\\mathring");
i(l, c, P, "", "\\@imath");
i(l, c, P, "", "\\@jmath");
i(l, c, g, "ı", "ı");
i(l, c, g, "ȷ", "ȷ");
i(M, c, g, "ı", "\\i", !0);
i(M, c, g, "ȷ", "\\j", !0);
i(M, c, g, "ß", "\\ss", !0);
i(M, c, g, "æ", "\\ae", !0);
i(M, c, g, "œ", "\\oe", !0);
i(M, c, g, "ø", "\\o", !0);
i(M, c, g, "Æ", "\\AE", !0);
i(M, c, g, "Œ", "\\OE", !0);
i(M, c, g, "Ø", "\\O", !0);
i(M, c, e0, "ˊ", "\\'");
i(M, c, e0, "ˋ", "\\`");
i(M, c, e0, "ˆ", "\\^");
i(M, c, e0, "˜", "\\~");
i(M, c, e0, "ˉ", "\\=");
i(M, c, e0, "˘", "\\u");
i(M, c, e0, "˙", "\\.");
i(M, c, e0, "¸", "\\c");
i(M, c, e0, "˚", "\\r");
i(M, c, e0, "ˇ", "\\v");
i(M, c, e0, "¨", '\\"');
i(M, c, e0, "˝", "\\H");
i(M, c, e0, "◯", "\\textcircled");
var ta = {
  "--": !0,
  "---": !0,
  "``": !0,
  "''": !0
};
i(M, c, g, "–", "--", !0);
i(M, c, g, "–", "\\textendash");
i(M, c, g, "—", "---", !0);
i(M, c, g, "—", "\\textemdash");
i(M, c, g, "‘", "`", !0);
i(M, c, g, "‘", "\\textquoteleft");
i(M, c, g, "’", "'", !0);
i(M, c, g, "’", "\\textquoteright");
i(M, c, g, "“", "``", !0);
i(M, c, g, "“", "\\textquotedblleft");
i(M, c, g, "”", "''", !0);
i(M, c, g, "”", "\\textquotedblright");
i(l, c, g, "°", "\\degree", !0);
i(M, c, g, "°", "\\degree");
i(M, c, g, "°", "\\textdegree", !0);
i(l, c, g, "£", "\\pounds");
i(l, c, g, "£", "\\mathsterling", !0);
i(M, c, g, "£", "\\pounds");
i(M, c, g, "£", "\\textsterling", !0);
i(l, f, g, "✠", "\\maltese");
i(M, f, g, "✠", "\\maltese");
var hr = '0123456789/@."';
for (var st = 0; st < hr.length; st++) {
  var dr = hr.charAt(st);
  i(l, c, g, dr, dr);
}
var mr = '0123456789!@*()-=+";:?/.,';
for (var lt = 0; lt < mr.length; lt++) {
  var fr = mr.charAt(lt);
  i(M, c, g, fr, fr);
}
var Ue = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";
for (var ut = 0; ut < Ue.length; ut++) {
  var Ie = Ue.charAt(ut);
  i(l, c, P, Ie, Ie), i(M, c, g, Ie, Ie);
}
i(l, f, g, "C", "ℂ");
i(M, f, g, "C", "ℂ");
i(l, f, g, "H", "ℍ");
i(M, f, g, "H", "ℍ");
i(l, f, g, "N", "ℕ");
i(M, f, g, "N", "ℕ");
i(l, f, g, "P", "ℙ");
i(M, f, g, "P", "ℙ");
i(l, f, g, "Q", "ℚ");
i(M, f, g, "Q", "ℚ");
i(l, f, g, "R", "ℝ");
i(M, f, g, "R", "ℝ");
i(l, f, g, "Z", "ℤ");
i(M, f, g, "Z", "ℤ");
i(l, c, P, "h", "ℎ");
i(M, c, P, "h", "ℎ");
var G;
for (var f0 = 0; f0 < Ue.length; f0++) {
  var n0 = Ue.charAt(f0);
  G = String.fromCharCode(55349, 56320 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56372 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56424 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56580 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56684 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56736 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56788 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56840 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56944 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), f0 < 26 && (G = String.fromCharCode(55349, 56632 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G), G = String.fromCharCode(55349, 56476 + f0), i(l, c, P, n0, G), i(M, c, g, n0, G));
}
G = "𝕜";
i(l, c, P, "k", G);
i(M, c, g, "k", G);
for (var ie = 0; ie < 10; ie++) {
  var K0 = ie.toString();
  G = String.fromCharCode(55349, 57294 + ie), i(l, c, P, K0, G), i(M, c, g, K0, G), G = String.fromCharCode(55349, 57314 + ie), i(l, c, P, K0, G), i(M, c, g, K0, G), G = String.fromCharCode(55349, 57324 + ie), i(l, c, P, K0, G), i(M, c, g, K0, G), G = String.fromCharCode(55349, 57334 + ie), i(l, c, P, K0, G), i(M, c, g, K0, G);
}
var Mt = "ÐÞþ";
for (var ot = 0; ot < Mt.length; ot++) {
  var Re = Mt.charAt(ot);
  i(l, c, P, Re, Re), i(M, c, g, Re, Re);
}
var Ft = {
  mathClass: "mathbf",
  textClass: "textbf",
  font: "Main-Bold"
}, pr = {
  mathClass: "mathnormal",
  textClass: "textit",
  font: "Math-Italic"
}, vr = {
  mathClass: "boldsymbol",
  textClass: "boldsymbol",
  font: "Main-BoldItalic"
}, Jn = {
  mathClass: "mathscr",
  textClass: "textscr",
  font: "Script-Regular"
}, le = {
  mathClass: "",
  textClass: "",
  font: ""
}, gr = {
  mathClass: "mathfrak",
  textClass: "textfrak",
  font: "Fraktur-Regular"
}, br = {
  mathClass: "mathbb",
  textClass: "textbb",
  font: "AMS-Regular"
}, yr = {
  mathClass: "mathboldfrak",
  textClass: "textboldfrak",
  font: "Fraktur-Regular"
}, Nt = {
  mathClass: "mathsf",
  textClass: "textsf",
  font: "SansSerif-Regular"
}, Ct = {
  mathClass: "mathboldsf",
  textClass: "textboldsf",
  font: "SansSerif-Bold"
}, xr = {
  mathClass: "mathitsf",
  textClass: "textitsf",
  font: "SansSerif-Italic"
}, Et = {
  mathClass: "mathtt",
  textClass: "texttt",
  font: "Typewriter-Regular"
}, wr = [
  Ft,
  Ft,
  // A-Z, a-z
  pr,
  pr,
  // A-Z, a-z
  vr,
  vr,
  // A-Z, a-z
  // Map fancy A-Z letters to script, not calligraphic.
  // This aligns with unicode-math and math fonts (except Cambria Math).
  Jn,
  le,
  // A-Z script, a-z — no font
  le,
  le,
  // A-Z bold script, a-z bold script — no font
  gr,
  gr,
  // A-Z, a-z
  br,
  br,
  // A-Z double-struck, k double-struck
  // Note that we are using a bold font, but font metrics for regular Fraktur.
  yr,
  yr,
  // A-Z, a-z
  Nt,
  Nt,
  // A-Z, a-z
  Ct,
  Ct,
  // A-Z, a-z
  xr,
  xr,
  // A-Z, a-z
  le,
  le,
  // A-Z bold italic sans, a-z bold italic sans - no font
  Et,
  Et
  // A-Z, a-z
], Qn = [
  Ft,
  // 0-9
  le,
  // 0-9 double-struck. No KaTeX font.
  Nt,
  // 0-9
  Ct,
  // 0-9
  Et
  // 0-9
], ei = (r) => {
  var e = r.charCodeAt(0), t = r.charCodeAt(1), a = (e - 55296) * 1024 + (t - 56320) + 65536;
  if (119808 <= a && a < 120484) {
    var n = Math.floor((a - 119808) / 26);
    return wr[n];
  } else if (120782 <= a && a <= 120831) {
    var s = Math.floor((a - 120782) / 10);
    return Qn[s];
  } else {
    if (a === 120485 || a === 120486)
      return wr[0];
    if (120486 < a && a < 120782)
      return le;
    throw new E("Unsupported character: " + r);
  }
}, Ve = function(e, t, a) {
  if (Q[a][e]) {
    var n = Q[a][e].replace;
    n && (e = n);
  }
  return {
    value: e,
    metrics: Xt(e, t, a)
  };
}, p0 = function(e, t, a, n, s) {
  var u = Ve(e, t, a), o = u.metrics;
  e = u.value;
  var h;
  if (o) {
    var m = o.italic;
    (a === "text" || n && n.font === "mathit") && (m = 0), h = new z0(e, o.height, o.depth, m, o.skew, o.width, s);
  } else
    typeof console < "u" && console.warn("No character metrics " + ("for '" + e + "' in style '" + t + "' and mode '" + a + "'")), h = new z0(e, 0, 0, 0, 0, 0, s);
  if (n) {
    h.maxFontSize = n.sizeMultiplier, n.style.isTight() && h.classes.push("mtight");
    var v = n.getColor();
    v && (h.style.color = v);
  }
  return h;
}, Vt = function(e, t, a, n) {
  return n === void 0 && (n = []), a.font === "boldsymbol" && Ve(e, "Main-Bold", t).metrics ? p0(e, "Main-Bold", t, a, n.concat(["mathbf"])) : e === "\\" || Q[t][e].font === "main" ? p0(e, "Main-Regular", t, a, n) : p0(e, "AMS-Regular", t, a, n.concat(["amsrm"]));
}, ti = function(e, t, a) {
  return a !== "textord" && Ve(e, "Math-BoldItalic", t).metrics ? {
    fontName: "Math-BoldItalic",
    fontClass: "boldsymbol"
  } : {
    fontName: "Main-Bold",
    fontClass: "mathbf"
  };
}, Ye = function(e, t) {
  var a = e.type === "mathord" ? "mathord" : "textord", n = e.mode, s = e.text, u = ["mord"], {
    font: o,
    fontFamily: h,
    fontWeight: m,
    fontShape: v
  } = t, b = n === "math" || n === "text" && !!o, k = b ? o : h, S = "", _ = "";
  if (s.charCodeAt(0) === 55349) {
    var C = ei(s);
    S = C.font, _ = C[n + "Class"];
  }
  if (S)
    return p0(s, S, n, t, u.concat(_));
  if (k) {
    var N, I;
    if (k === "boldsymbol") {
      var x = ti(s, n, a);
      N = x.fontName, I = [x.fontClass];
    } else b ? (N = Dt[o].fontName, I = [o]) : (N = qe(h, m, v), I = [h, m, v]);
    if (Ve(s, N, n).metrics)
      return p0(s, N, n, t, u.concat(I));
    if (ta.hasOwnProperty(s) && N.slice(0, 10) === "Typewriter") {
      for (var w = [], A = 0; A < s.length; A++)
        w.push(p0(s[A], N, n, t, u.concat(I)));
      return X0(w);
    }
  }
  if (a === "mathord")
    return p0(s, "Math-Italic", n, t, u.concat(["mathnormal"]));
  if (a === "textord") {
    var T = Q[n][s] && Q[n][s].font;
    if (T === "ams") {
      var z = qe("amsrm", m, v);
      return p0(s, z, n, t, u.concat("amsrm", m, v));
    } else if (T === "main" || !T) {
      var q = qe("textrm", m, v);
      return p0(s, q, n, t, u.concat(m, v));
    } else {
      var H = qe(T, m, v);
      return p0(s, H, n, t, u.concat(H, m, v));
    }
  } else
    throw new Error("unexpected type: " + a + " in makeOrd");
}, ri = (r, e) => {
  if (Q0(r.classes) !== Q0(e.classes) || r.skew !== e.skew || r.maxFontSize !== e.maxFontSize || r.italic !== 0 && r.hasClass("mathnormal"))
    return !1;
  if (r.classes.length === 1) {
    var t = r.classes[0];
    if (t === "mbin" || t === "mord")
      return !1;
  }
  for (var a of Object.keys(r.style))
    if (r.style[a] !== e.style[a])
      return !1;
  for (var n of Object.keys(e.style))
    if (r.style[n] !== e.style[n])
      return !1;
  return !0;
}, ra = (r) => {
  for (var e = 0; e < r.length - 1; e++) {
    var t = r[e], a = r[e + 1];
    t instanceof z0 && a instanceof z0 && ri(t, a) && (t.text += a.text, t.height = Math.max(t.height, a.height), t.depth = Math.max(t.depth, a.depth), t.italic = a.italic, r.splice(e + 1, 1), e--);
  }
  return r;
}, Yt = function(e) {
  for (var t = 0, a = 0, n = 0, s = 0; s < e.children.length; s++) {
    var u = e.children[s];
    u.height > t && (t = u.height), u.depth > a && (a = u.depth), u.maxFontSize > n && (n = u.maxFontSize);
  }
  e.height = t, e.depth = a, e.maxFontSize = n;
}, F = function(e, t, a, n) {
  var s = new be(e, t, a, n);
  return Yt(s), s;
}, te = (r, e, t, a) => new be(r, e, t, a), pe = function(e, t, a) {
  var n = F([e], [], t);
  return n.height = Math.max(a || t.fontMetrics().defaultRuleThickness, t.minRuleThickness), n.style.borderBottomWidth = B(n.height), n.maxFontSize = 1, n;
}, ai = function(e, t, a, n) {
  var s = new Xe(e, t, a, n);
  return Yt(s), s;
}, X0 = function(e) {
  var t = new ge(e);
  return Yt(t), t;
}, ve = function(e, t) {
  return e instanceof ge ? F([], [e], t) : e;
}, ni = function(e) {
  if (e.positionType === "individualShift") {
    for (var t = e.children, a = [t[0]], n = -t[0].shift - t[0].elem.depth, s = n, u = 1; u < t.length; u++) {
      var o = -t[u].shift - s - t[u].elem.depth, h = o - (t[u - 1].elem.height + t[u - 1].elem.depth);
      s = s + o, a.push({
        type: "kern",
        size: h
      }), a.push(t[u]);
    }
    return {
      children: a,
      depth: n
    };
  }
  var m;
  if (e.positionType === "top") {
    for (var v = e.positionData, b = 0; b < e.children.length; b++) {
      var k = e.children[b];
      v -= k.type === "kern" ? k.size : k.elem.height + k.elem.depth;
    }
    m = v;
  } else if (e.positionType === "bottom")
    m = -e.positionData;
  else {
    var S = e.children[0];
    if (S.type !== "elem")
      throw new Error('First child must have type "elem".');
    if (e.positionType === "shift")
      m = -S.elem.depth - e.positionData;
    else if (e.positionType === "firstBaseline")
      m = -S.elem.depth;
    else
      throw new Error("Invalid positionType " + e.positionType + ".");
  }
  return {
    children: e.children,
    depth: m
  };
}, V = function(e, t) {
  for (var {
    children: a,
    depth: n
  } = ni(e), s = 0, u = 0; u < a.length; u++) {
    var o = a[u];
    if (o.type === "elem") {
      var h = o.elem;
      s = Math.max(s, h.maxFontSize, h.height);
    }
  }
  s += 2;
  var m = F(["pstrut"], []);
  m.style.height = B(s);
  for (var v = [], b = n, k = n, S = n, _ = 0; _ < a.length; _++) {
    var C = a[_];
    if (C.type === "kern")
      S += C.size;
    else {
      var N = C.elem, I = C.wrapperClasses || [], x = C.wrapperStyle || {}, w = F(I, [m, N], void 0, x);
      w.style.top = B(-s - S - N.depth), C.marginLeft && (w.style.marginLeft = C.marginLeft), C.marginRight && (w.style.marginRight = C.marginRight), v.push(w), S += N.height + N.depth;
    }
    b = Math.min(b, S), k = Math.max(k, S);
  }
  var A = F(["vlist"], v);
  A.style.height = B(k);
  var T;
  if (b < 0) {
    var z = F([], []), q = F(["vlist"], [z]);
    q.style.height = B(-b);
    var H = F(["vlist-s"], [new z0("​")]);
    T = [F(["vlist-r"], [A, H]), F(["vlist-r"], [q])];
  } else
    T = [F(["vlist-r"], [A])];
  var R = F(["vlist-t"], T);
  return T.length === 2 && R.classes.push("vlist-t2"), R.height = k, R.depth = -b, R;
}, aa = (r, e) => {
  var t = F(["mspace"], [], e), a = a0(r, e);
  return t.style.marginRight = B(a), t;
}, qe = (r, e, t) => {
  var a, n;
  switch (r) {
    case "amsrm":
      a = "AMS";
      break;
    case "textrm":
      a = "Main";
      break;
    case "textsf":
      a = "SansSerif";
      break;
    case "texttt":
      a = "Typewriter";
      break;
    default:
      a = r;
  }
  return e === "textbf" && t === "textit" ? n = "BoldItalic" : e === "textbf" ? n = "Bold" : t === "textit" ? n = "Italic" : n = "Regular", a + "-" + n;
}, Dt = {
  // styles
  mathbf: {
    variant: "bold",
    fontName: "Main-Bold"
  },
  mathrm: {
    variant: "normal",
    fontName: "Main-Regular"
  },
  textit: {
    variant: "italic",
    fontName: "Main-Italic"
  },
  mathit: {
    variant: "italic",
    fontName: "Main-Italic"
  },
  mathnormal: {
    variant: "italic",
    fontName: "Math-Italic"
  },
  mathsfit: {
    variant: "sans-serif-italic",
    fontName: "SansSerif-Italic"
  },
  // "boldsymbol" is missing because they require the use of multiple fonts:
  // Math-BoldItalic and Main-Bold.  This is handled by a special case in
  // makeOrd which ends up calling boldsymbol.
  // families
  mathbb: {
    variant: "double-struck",
    fontName: "AMS-Regular"
  },
  mathcal: {
    variant: "script",
    fontName: "Caligraphic-Regular"
  },
  mathfrak: {
    variant: "fraktur",
    fontName: "Fraktur-Regular"
  },
  mathscr: {
    variant: "script",
    fontName: "Script-Regular"
  },
  mathsf: {
    variant: "sans-serif",
    fontName: "SansSerif-Regular"
  },
  mathtt: {
    variant: "monospace",
    fontName: "Typewriter-Regular"
  }
}, na = {
  //   path, width, height
  vec: ["vec", 0.471, 0.714],
  // values from the font glyph
  oiintSize1: ["oiintSize1", 0.957, 0.499],
  // oval to overlay the integrand
  oiintSize2: ["oiintSize2", 1.472, 0.659],
  oiiintSize1: ["oiiintSize1", 1.304, 0.499],
  oiiintSize2: ["oiiintSize2", 1.98, 0.659]
}, ia = function(e, t) {
  var [a, n, s] = na[e], u = new ee(a), o = new G0([u], {
    width: B(n),
    height: B(s),
    // Override CSS rule `.katex svg { width: 100% }`
    style: "width:" + B(n),
    viewBox: "0 0 " + 1e3 * n + " " + 1e3 * s,
    preserveAspectRatio: "xMinYMin"
  }), h = te(["overlay"], [o], t);
  return h.height = s, h.style.height = B(s), h.style.width = B(n), h;
}, r0 = {
  number: 3,
  unit: "mu"
}, se = {
  number: 4,
  unit: "mu"
}, H0 = {
  number: 5,
  unit: "mu"
}, ii = {
  mord: {
    mop: r0,
    mbin: se,
    mrel: H0,
    minner: r0
  },
  mop: {
    mord: r0,
    mop: r0,
    mrel: H0,
    minner: r0
  },
  mbin: {
    mord: se,
    mop: se,
    mopen: se,
    minner: se
  },
  mrel: {
    mord: H0,
    mop: H0,
    mopen: H0,
    minner: H0
  },
  mopen: {},
  mclose: {
    mop: r0,
    mbin: se,
    mrel: H0,
    minner: r0
  },
  mpunct: {
    mord: r0,
    mop: r0,
    mrel: H0,
    mopen: r0,
    mclose: r0,
    mpunct: r0,
    minner: r0
  },
  minner: {
    mord: r0,
    mop: r0,
    mbin: se,
    mrel: H0,
    mopen: r0,
    mpunct: r0,
    minner: r0
  }
}, si = {
  mord: {
    mop: r0
  },
  mop: {
    mord: r0,
    mop: r0
  },
  mbin: {},
  mrel: {},
  mopen: {},
  mclose: {
    mop: r0
  },
  mpunct: {},
  minner: {
    mop: r0
  }
}, sa = {}, ke = {}, Ae = {};
function O(r) {
  for (var {
    type: e,
    names: t,
    htmlBuilder: a,
    mathmlBuilder: n
  } = r, s = 0; s < t.length; ++s)
    sa[t[s]] = r;
  e && (a && (ke[e] = a), n && (Ae[e] = n));
}
function ue(r) {
  var {
    type: e,
    htmlBuilder: t,
    mathmlBuilder: a
  } = r;
  t && (ke[e] = t), a && (Ae[e] = a);
}
var je = function(e) {
  return e.type === "ordgroup" && e.body.length === 1 ? e.body[0] : e;
}, i0 = function(e) {
  return e.type === "ordgroup" ? e.body : [e];
}, li = /* @__PURE__ */ new Set(["leftmost", "mbin", "mopen", "mrel", "mop", "mpunct"]), ui = /* @__PURE__ */ new Set(["rightmost", "mrel", "mclose", "mpunct"]), oi = {
  display: U.DISPLAY,
  text: U.TEXT,
  script: U.SCRIPT,
  scriptscript: U.SCRIPTSCRIPT
}, ci = {
  mord: "mord",
  mop: "mop",
  mbin: "mbin",
  mrel: "mrel",
  mopen: "mopen",
  mclose: "mclose",
  mpunct: "mpunct",
  minner: "minner"
}, o0 = function(e, t, a, n) {
  n === void 0 && (n = [null, null]);
  for (var s = [], u = 0; u < e.length; u++) {
    var o = Y(e[u], t);
    if (o instanceof ge) {
      var h = o.children;
      s.push(...h);
    } else
      s.push(o);
  }
  if (ra(s), !a)
    return s;
  var m = t;
  if (e.length === 1) {
    var v = e[0];
    v.type === "sizing" ? m = t.havingSize(v.size) : v.type === "styling" && (m = t.havingStyle(oi[v.style]));
  }
  var b = F([n[0] || "leftmost"], [], t), k = F([n[1] || "rightmost"], [], t), S = a === "root";
  return Bt(s, (_, C) => {
    var N = C.classes[0], I = _.classes[0];
    N === "mbin" && ui.has(I) ? C.classes[0] = "mord" : I === "mbin" && li.has(N) && (_.classes[0] = "mord");
  }, {
    node: b
  }, k, S), Bt(s, (_, C) => {
    var N, I, x = Rt(C), w = Rt(_), A = x && w ? _.hasClass("mtight") ? (N = si[x]) == null ? void 0 : N[w] : (I = ii[x]) == null ? void 0 : I[w] : null;
    if (A)
      return aa(A, m);
  }, {
    node: b
  }, k, S), s;
}, Bt = function(e, t, a, n, s) {
  n && e.push(n);
  for (var u = 0; u < e.length; u++) {
    var o = e[u], h = la(o);
    if (h) {
      Bt(h.children, t, a, null, s);
      continue;
    }
    var m = !o.hasClass("mspace");
    if (m) {
      var v = t(o, a.node);
      v && (a.insertAfter ? a.insertAfter(v) : (e.unshift(v), u++));
    }
    m ? a.node = o : s && o.hasClass("newline") && (a.node = F(["leftmost"])), a.insertAfter = /* @__PURE__ */ ((b) => (k) => {
      e.splice(b + 1, 0, k), u++;
    })(u);
  }
  n && e.pop();
}, la = function(e) {
  return e instanceof ge || e instanceof Xe || e instanceof be && e.hasClass("enclosing") ? e : null;
}, It = function(e, t) {
  var a = la(e);
  if (a) {
    var n = a.children;
    if (n.length) {
      if (t === "right")
        return It(n[n.length - 1], "right");
      if (t === "left")
        return It(n[0], "left");
    }
  }
  return e;
}, Rt = function(e, t) {
  if (!e)
    return null;
  t && (e = It(e, t));
  var a = e.classes[0];
  return ci[a] || null;
}, Te = function(e, t) {
  var a = ["nulldelimiter"].concat(e.baseSizingClasses());
  return F(t.concat(a));
}, Y = function(e, t, a) {
  if (!e)
    return F();
  if (ke[e.type]) {
    var n = ke[e.type](e, t);
    if (a && t.size !== a.size) {
      n = F(t.sizingClasses(a), [n], t);
      var s = t.sizeMultiplier / a.sizeMultiplier;
      n.height *= s, n.depth *= s;
    }
    return n;
  } else
    throw new E("Got group of unknown type: '" + e.type + "'");
};
function _e(r, e) {
  var t = F(["base"], r, e), a = F(["strut"]);
  return a.style.height = B(t.height + t.depth), t.depth && (a.style.verticalAlign = B(-t.depth)), t.children.unshift(a), t;
}
function qt(r, e) {
  var t = null;
  r.length === 1 && r[0].type === "tag" && (t = r[0].tag, r = r[0].body);
  var a = o0(r, e, "root"), n;
  a.length === 2 && a[1].hasClass("tag") && (n = a.pop());
  for (var s = [], u = [], o = 0; o < a.length; o++)
    if (u.push(a[o]), a[o].hasClass("mbin") || a[o].hasClass("mrel") || a[o].hasClass("allowbreak")) {
      for (var h = !1; o < a.length - 1 && a[o + 1].hasClass("mspace") && !a[o + 1].hasClass("newline"); )
        o++, u.push(a[o]), a[o].hasClass("nobreak") && (h = !0);
      h || (s.push(_e(u, e)), u = []);
    } else a[o].hasClass("newline") && (u.pop(), u.length > 0 && (s.push(_e(u, e)), u = []), s.push(a[o]));
  u.length > 0 && s.push(_e(u, e));
  var m;
  t ? (m = _e(o0(t, e, !0), e), m.classes = ["tag"], s.push(m)) : n && s.push(n);
  var v = F(["katex-html"], s);
  if (v.setAttribute("aria-hidden", "true"), m) {
    var b = m.children[0];
    b.style.height = B(v.height + v.depth), v.depth && (b.style.verticalAlign = B(-v.depth));
  }
  return v;
}
function ua(r) {
  return new ge(r);
}
class D {
  constructor(e, t, a) {
    this.type = void 0, this.attributes = void 0, this.children = void 0, this.classes = void 0, this.type = e, this.attributes = {}, this.children = t || [], this.classes = a || [];
  }
  /**
   * Sets an attribute on a MathML node. MathML depends on attributes to convey a
   * semantic content, so this is used heavily.
   */
  setAttribute(e, t) {
    this.attributes[e] = t;
  }
  /**
   * Gets an attribute on a MathML node.
   */
  getAttribute(e) {
    return this.attributes[e];
  }
  /**
   * Converts the math node into a MathML-namespaced DOM element.
   */
  toNode() {
    var e = document.createElementNS("http://www.w3.org/1998/Math/MathML", this.type);
    for (var t in this.attributes)
      Object.prototype.hasOwnProperty.call(this.attributes, t) && e.setAttribute(t, this.attributes[t]);
    this.classes.length > 0 && (e.className = Q0(this.classes));
    for (var a = 0; a < this.children.length; a++)
      if (this.children[a] instanceof s0 && this.children[a + 1] instanceof s0) {
        for (var n = this.children[a].toText() + this.children[++a].toText(); this.children[a + 1] instanceof s0; )
          n += this.children[++a].toText();
        e.appendChild(new s0(n).toNode());
      } else
        e.appendChild(this.children[a].toNode());
    return e;
  }
  /**
   * Converts the math node into an HTML markup string.
   */
  toMarkup() {
    var e = "<" + this.type;
    for (var t in this.attributes)
      Object.prototype.hasOwnProperty.call(this.attributes, t) && (e += " " + t + '="', e += d0(this.attributes[t]), e += '"');
    this.classes.length > 0 && (e += ' class ="' + d0(Q0(this.classes)) + '"'), e += ">";
    for (var a = 0; a < this.children.length; a++)
      e += this.children[a].toMarkup();
    return e += "</" + this.type + ">", e;
  }
  /**
   * Converts the math node into a string, similar to innerText, but escaped.
   */
  toText() {
    return this.children.map((e) => e.toText()).join("");
  }
}
class s0 {
  constructor(e) {
    this.text = void 0, this.text = e;
  }
  /**
   * Converts the text node into a DOM text node.
   */
  toNode() {
    return document.createTextNode(this.text);
  }
  /**
   * Converts the text node into escaped HTML markup
   * (representing the text itself).
   */
  toMarkup() {
    return d0(this.toText());
  }
  /**
   * Converts the text node into a string
   * (representing the text itself).
   */
  toText() {
    return this.text;
  }
}
class oa {
  /**
   * Create a Space node with width given in CSS ems.
   */
  constructor(e) {
    this.width = void 0, this.character = void 0, this.width = e, e >= 0.05555 && e <= 0.05556 ? this.character = " " : e >= 0.1666 && e <= 0.1667 ? this.character = " " : e >= 0.2222 && e <= 0.2223 ? this.character = " " : e >= 0.2777 && e <= 0.2778 ? this.character = "  " : e >= -0.05556 && e <= -0.05555 ? this.character = " ⁣" : e >= -0.1667 && e <= -0.1666 ? this.character = " ⁣" : e >= -0.2223 && e <= -0.2222 ? this.character = " ⁣" : e >= -0.2778 && e <= -0.2777 ? this.character = " ⁣" : this.character = null;
  }
  /**
   * Converts the math node into a MathML-namespaced DOM element.
   */
  toNode() {
    if (this.character)
      return document.createTextNode(this.character);
    var e = document.createElementNS("http://www.w3.org/1998/Math/MathML", "mspace");
    return e.setAttribute("width", B(this.width)), e;
  }
  /**
   * Converts the math node into an HTML markup string.
   */
  toMarkup() {
    return this.character ? "<mtext>" + this.character + "</mtext>" : '<mspace width="' + B(this.width) + '"/>';
  }
  /**
   * Converts the math node into a string, similar to innerText.
   */
  toText() {
    return this.character ? this.character : " ";
  }
}
var hi = /* @__PURE__ */ new Set(["\\imath", "\\jmath"]), di = /* @__PURE__ */ new Set(["mrow", "mtable"]), C0 = function(e, t, a) {
  return Q[t][e] && Q[t][e].replace && e.charCodeAt(0) !== 55349 && !(ta.hasOwnProperty(e) && a && (a.fontFamily && a.fontFamily.slice(4, 6) === "tt" || a.font && a.font.slice(4, 6) === "tt")) && (e = Q[t][e].replace), new s0(e);
}, Wt = function(e) {
  return e.length === 1 ? e[0] : new D("mrow", e);
}, mi = {
  mathit: "italic",
  boldsymbol: (r) => r.type === "textord" ? "bold" : "bold-italic",
  mathbf: "bold",
  mathbb: "double-struck",
  mathsfit: "sans-serif-italic",
  mathfrak: "fraktur",
  mathscr: "script",
  mathcal: "script",
  mathsf: "sans-serif",
  mathtt: "monospace"
}, Zt = (r, e) => {
  if (r.mode === "text") {
    if (e.fontFamily === "texttt")
      return "monospace";
    if (e.fontFamily === "textsf")
      return e.fontShape === "textit" && e.fontWeight === "textbf" ? "sans-serif-bold-italic" : e.fontShape === "textit" ? "sans-serif-italic" : e.fontWeight === "textbf" ? "bold-sans-serif" : "sans-serif";
    if (e.fontShape === "textit" && e.fontWeight === "textbf")
      return "bold-italic";
    if (e.fontShape === "textit")
      return "italic";
    if (e.fontWeight === "textbf")
      return "bold";
  }
  var t = e.font;
  if (!t || t === "mathnormal")
    return null;
  var a = r.mode, n = mi[t];
  if (n)
    return typeof n == "function" ? n(r) : n;
  var s = r.text;
  if (hi.has(s))
    return null;
  if (Q[a][s]) {
    var u = Q[a][s].replace;
    u && (s = u);
  }
  var o = Dt[t].fontName;
  return Xt(s, o, a) ? Dt[t].variant : null;
};
function ct(r) {
  if (!r)
    return !1;
  if (r.type === "mi" && r.children.length === 1) {
    var e = r.children[0];
    return e instanceof s0 && e.text === ".";
  } else if (r.type === "mo" && r.children.length === 1 && r.getAttribute("separator") === "true" && r.getAttribute("lspace") === "0em" && r.getAttribute("rspace") === "0em") {
    var t = r.children[0];
    return t instanceof s0 && t.text === ",";
  } else
    return !1;
}
var F0 = function(e, t, a) {
  if (e.length === 1) {
    var n = K(e[0], t);
    return a && n instanceof D && n.type === "mo" && (n.setAttribute("lspace", "0em"), n.setAttribute("rspace", "0em")), [n];
  }
  for (var s = [], u, o = 0; o < e.length; o++) {
    var h = K(e[o], t);
    if (h instanceof D && u instanceof D) {
      if (h.type === "mtext" && u.type === "mtext" && h.getAttribute("mathvariant") === u.getAttribute("mathvariant")) {
        u.children.push(...h.children);
        continue;
      } else if (h.type === "mn" && u.type === "mn") {
        u.children.push(...h.children);
        continue;
      } else if (ct(h) && u.type === "mn") {
        u.children.push(...h.children);
        continue;
      } else if (h.type === "mn" && ct(u))
        h.children = [...u.children, ...h.children], s.pop();
      else if ((h.type === "msup" || h.type === "msub") && h.children.length >= 1 && (u.type === "mn" || ct(u))) {
        var m = h.children[0];
        m instanceof D && m.type === "mn" && (m.children = [...u.children, ...m.children], s.pop());
      } else if (u.type === "mi" && u.children.length === 1) {
        var v = u.children[0];
        if (v instanceof s0 && v.text === "̸" && (h.type === "mo" || h.type === "mi" || h.type === "mn")) {
          var b = h.children[0];
          b instanceof s0 && b.text.length > 0 && (b.text = b.text.slice(0, 1) + "̸" + b.text.slice(1), s.pop());
        }
      }
    }
    s.push(h), u = h;
  }
  return s;
}, re = function(e, t, a) {
  return Wt(F0(e, t, a));
}, K = function(e, t) {
  if (!e)
    return new D("mrow");
  if (Ae[e.type])
    return Ae[e.type](e, t);
  throw new E("Got group of unknown type: '" + e.type + "'");
};
function Sr(r, e, t, a, n) {
  var s = F0(r, t), u;
  s.length === 1 && s[0] instanceof D && di.has(s[0].type) ? u = s[0] : u = new D("mrow", s);
  var o = new D("annotation", [new s0(e)]);
  o.setAttribute("encoding", "application/x-tex");
  var h = new D("semantics", [u, o]), m = new D("math", [h]);
  m.setAttribute("xmlns", "http://www.w3.org/1998/Math/MathML"), a && m.setAttribute("display", "block");
  var v = n ? "katex" : "katex-mathml";
  return F([v], [m]);
}
var fi = [
  // Each element contains [textsize, scriptsize, scriptscriptsize].
  // The size mappings are taken from TeX with \normalsize=10pt.
  [1, 1, 1],
  // size1: [5, 5, 5]              \tiny
  [2, 1, 1],
  // size2: [6, 5, 5]
  [3, 1, 1],
  // size3: [7, 5, 5]              \scriptsize
  [4, 2, 1],
  // size4: [8, 6, 5]              \footnotesize
  [5, 2, 1],
  // size5: [9, 6, 5]              \small
  [6, 3, 1],
  // size6: [10, 7, 5]             \normalsize
  [7, 4, 2],
  // size7: [12, 8, 6]             \large
  [8, 6, 3],
  // size8: [14.4, 10, 7]          \Large
  [9, 7, 6],
  // size9: [17.28, 12, 10]        \LARGE
  [10, 8, 7],
  // size10: [20.74, 14.4, 12]     \huge
  [11, 10, 9]
  // size11: [24.88, 20.74, 17.28] \HUGE
], kr = [
  // fontMetrics.js:getGlobalMetrics also uses size indexes, so if
  // you change size indexes, change that function.
  0.5,
  0.6,
  0.7,
  0.8,
  0.9,
  1,
  1.2,
  1.44,
  1.728,
  2.074,
  2.488
], Ar = function(e, t) {
  return t.size < 2 ? e : fi[e - 1][t.size - 1];
};
class $0 {
  constructor(e) {
    this.style = void 0, this.color = void 0, this.size = void 0, this.textSize = void 0, this.phantom = void 0, this.font = void 0, this.fontFamily = void 0, this.fontWeight = void 0, this.fontShape = void 0, this.sizeMultiplier = void 0, this.maxSize = void 0, this.minRuleThickness = void 0, this._fontMetrics = void 0, this.style = e.style, this.color = e.color, this.size = e.size || $0.BASESIZE, this.textSize = e.textSize || this.size, this.phantom = !!e.phantom, this.font = e.font || "", this.fontFamily = e.fontFamily || "", this.fontWeight = e.fontWeight || "", this.fontShape = e.fontShape || "", this.sizeMultiplier = kr[this.size - 1], this.maxSize = e.maxSize, this.minRuleThickness = e.minRuleThickness, this._fontMetrics = void 0;
  }
  /**
   * Returns a new options object with the same properties as "this".  Properties
   * from "extension" will be copied to the new options object.
   */
  extend(e) {
    var t = {
      style: this.style,
      size: this.size,
      textSize: this.textSize,
      color: this.color,
      phantom: this.phantom,
      font: this.font,
      fontFamily: this.fontFamily,
      fontWeight: this.fontWeight,
      fontShape: this.fontShape,
      maxSize: this.maxSize,
      minRuleThickness: this.minRuleThickness
    };
    return Object.assign(t, e), new $0(t);
  }
  /**
   * Return an options object with the given style. If `this.style === style`,
   * returns `this`.
   */
  havingStyle(e) {
    return this.style === e ? this : this.extend({
      style: e,
      size: Ar(this.textSize, e)
    });
  }
  /**
   * Return an options object with a cramped version of the current style. If
   * the current style is cramped, returns `this`.
   */
  havingCrampedStyle() {
    return this.havingStyle(this.style.cramp());
  }
  /**
   * Return an options object with the given size and in at least `\textstyle`.
   * Returns `this` if appropriate.
   */
  havingSize(e) {
    return this.size === e && this.textSize === e ? this : this.extend({
      style: this.style.text(),
      size: e,
      textSize: e,
      sizeMultiplier: kr[e - 1]
    });
  }
  /**
   * Like `this.havingSize(BASESIZE).havingStyle(style)`. If `style` is omitted,
   * changes to at least `\textstyle`.
   */
  havingBaseStyle(e) {
    e = e || this.style.text();
    var t = Ar($0.BASESIZE, e);
    return this.size === t && this.textSize === $0.BASESIZE && this.style === e ? this : this.extend({
      style: e,
      size: t
    });
  }
  /**
   * Remove the effect of sizing changes such as \Huge.
   * Keep the effect of the current style, such as \scriptstyle.
   */
  havingBaseSizing() {
    var e;
    switch (this.style.id) {
      case 4:
      case 5:
        e = 3;
        break;
      case 6:
      case 7:
        e = 1;
        break;
      default:
        e = 6;
    }
    return this.extend({
      style: this.style.text(),
      size: e
    });
  }
  /**
   * Create a new options object with the given color.
   */
  withColor(e) {
    return this.extend({
      color: e
    });
  }
  /**
   * Create a new options object with "phantom" set to true.
   */
  withPhantom() {
    return this.extend({
      phantom: !0
    });
  }
  /**
   * Creates a new options object with the given math font or old text font.
   * @type {[type]}
   */
  withFont(e) {
    return this.extend({
      font: e
    });
  }
  /**
   * Create a new options objects with the given fontFamily.
   */
  withTextFontFamily(e) {
    return this.extend({
      fontFamily: e,
      font: ""
    });
  }
  /**
   * Creates a new options object with the given font weight
   */
  withTextFontWeight(e) {
    return this.extend({
      fontWeight: e,
      font: ""
    });
  }
  /**
   * Creates a new options object with the given font weight
   */
  withTextFontShape(e) {
    return this.extend({
      fontShape: e,
      font: ""
    });
  }
  /**
   * Return the CSS sizing classes required to switch from enclosing options
   * `oldOptions` to `this`. Returns an array of classes.
   */
  sizingClasses(e) {
    return e.size !== this.size ? ["sizing", "reset-size" + e.size, "size" + this.size] : [];
  }
  /**
   * Return the CSS sizing classes required to switch to the base size. Like
   * `this.havingSize(BASESIZE).sizingClasses(this)`.
   */
  baseSizingClasses() {
    return this.size !== $0.BASESIZE ? ["sizing", "reset-size" + this.size, "size" + $0.BASESIZE] : [];
  }
  /**
   * Return the font metrics for this size.
   */
  fontMetrics() {
    return this._fontMetrics || (this._fontMetrics = Kn(this.size)), this._fontMetrics;
  }
  /**
   * Gets the CSS color of the current options object
   */
  getColor() {
    return this.phantom ? "transparent" : this.color;
  }
}
$0.BASESIZE = 6;
var ca = function(e) {
  return new $0({
    style: e.displayMode ? U.DISPLAY : U.TEXT,
    maxSize: e.maxSize,
    minRuleThickness: e.minRuleThickness
  });
}, ha = function(e, t) {
  if (t.displayMode) {
    var a = ["katex-display"];
    t.leqno && a.push("leqno"), t.fleqn && a.push("fleqn"), e = F(a, [e]);
  }
  return e;
}, pi = function(e, t, a) {
  var n = ca(a), s;
  if (a.output === "mathml")
    return Sr(e, t, n, a.displayMode, !0);
  if (a.output === "html") {
    var u = qt(e, n);
    s = F(["katex"], [u]);
  } else {
    var o = Sr(e, t, n, a.displayMode, !1), h = qt(e, n);
    s = F(["katex"], [o, h]);
  }
  return ha(s, a);
}, vi = function(e, t, a) {
  var n = ca(a), s = qt(e, n), u = F(["katex"], [s]);
  return ha(u, a);
}, gi = {
  widehat: "^",
  widecheck: "ˇ",
  widetilde: "~",
  utilde: "~",
  overleftarrow: "←",
  underleftarrow: "←",
  xleftarrow: "←",
  overrightarrow: "→",
  underrightarrow: "→",
  xrightarrow: "→",
  underbrace: "⏟",
  overbrace: "⏞",
  underbracket: "⎵",
  overbracket: "⎴",
  overgroup: "⏠",
  undergroup: "⏡",
  overleftrightarrow: "↔",
  underleftrightarrow: "↔",
  xleftrightarrow: "↔",
  Overrightarrow: "⇒",
  xRightarrow: "⇒",
  overleftharpoon: "↼",
  xleftharpoonup: "↼",
  overrightharpoon: "⇀",
  xrightharpoonup: "⇀",
  xLeftarrow: "⇐",
  xLeftrightarrow: "⇔",
  xhookleftarrow: "↩",
  xhookrightarrow: "↪",
  xmapsto: "↦",
  xrightharpoondown: "⇁",
  xleftharpoondown: "↽",
  xrightleftharpoons: "⇌",
  xleftrightharpoons: "⇋",
  xtwoheadleftarrow: "↞",
  xtwoheadrightarrow: "↠",
  xlongequal: "=",
  xtofrom: "⇄",
  xrightleftarrows: "⇄",
  xrightequilibrium: "⇌",
  // Not a perfect match.
  xleftequilibrium: "⇋",
  // None better available.
  "\\cdrightarrow": "→",
  "\\cdleftarrow": "←",
  "\\cdlongequal": "="
}, We = function(e) {
  var t = new D("mo", [new s0(gi[e.replace(/^\\/, "")])]);
  return t.setAttribute("stretchy", "true"), t;
}, bi = {
  //   path(s), minWidth, height, align
  overrightarrow: [["rightarrow"], 0.888, 522, "xMaxYMin"],
  overleftarrow: [["leftarrow"], 0.888, 522, "xMinYMin"],
  underrightarrow: [["rightarrow"], 0.888, 522, "xMaxYMin"],
  underleftarrow: [["leftarrow"], 0.888, 522, "xMinYMin"],
  xrightarrow: [["rightarrow"], 1.469, 522, "xMaxYMin"],
  "\\cdrightarrow": [["rightarrow"], 3, 522, "xMaxYMin"],
  // CD minwwidth2.5pc
  xleftarrow: [["leftarrow"], 1.469, 522, "xMinYMin"],
  "\\cdleftarrow": [["leftarrow"], 3, 522, "xMinYMin"],
  Overrightarrow: [["doublerightarrow"], 0.888, 560, "xMaxYMin"],
  xRightarrow: [["doublerightarrow"], 1.526, 560, "xMaxYMin"],
  xLeftarrow: [["doubleleftarrow"], 1.526, 560, "xMinYMin"],
  overleftharpoon: [["leftharpoon"], 0.888, 522, "xMinYMin"],
  xleftharpoonup: [["leftharpoon"], 0.888, 522, "xMinYMin"],
  xleftharpoondown: [["leftharpoondown"], 0.888, 522, "xMinYMin"],
  overrightharpoon: [["rightharpoon"], 0.888, 522, "xMaxYMin"],
  xrightharpoonup: [["rightharpoon"], 0.888, 522, "xMaxYMin"],
  xrightharpoondown: [["rightharpoondown"], 0.888, 522, "xMaxYMin"],
  xlongequal: [["longequal"], 0.888, 334, "xMinYMin"],
  "\\cdlongequal": [["longequal"], 3, 334, "xMinYMin"],
  xtwoheadleftarrow: [["twoheadleftarrow"], 0.888, 334, "xMinYMin"],
  xtwoheadrightarrow: [["twoheadrightarrow"], 0.888, 334, "xMaxYMin"],
  overleftrightarrow: [["leftarrow", "rightarrow"], 0.888, 522],
  overbrace: [["leftbrace", "midbrace", "rightbrace"], 1.6, 548],
  underbrace: [["leftbraceunder", "midbraceunder", "rightbraceunder"], 1.6, 548],
  underleftrightarrow: [["leftarrow", "rightarrow"], 0.888, 522],
  xleftrightarrow: [["leftarrow", "rightarrow"], 1.75, 522],
  xLeftrightarrow: [["doubleleftarrow", "doublerightarrow"], 1.75, 560],
  xrightleftharpoons: [["leftharpoondownplus", "rightharpoonplus"], 1.75, 716],
  xleftrightharpoons: [["leftharpoonplus", "rightharpoondownplus"], 1.75, 716],
  xhookleftarrow: [["leftarrow", "righthook"], 1.08, 522],
  xhookrightarrow: [["lefthook", "rightarrow"], 1.08, 522],
  overlinesegment: [["leftlinesegment", "rightlinesegment"], 0.888, 522],
  underlinesegment: [["leftlinesegment", "rightlinesegment"], 0.888, 522],
  overbracket: [["leftbracketover", "rightbracketover"], 1.6, 440],
  underbracket: [["leftbracketunder", "rightbracketunder"], 1.6, 410],
  overgroup: [["leftgroup", "rightgroup"], 0.888, 342],
  undergroup: [["leftgroupunder", "rightgroupunder"], 0.888, 342],
  xmapsto: [["leftmapsto", "rightarrow"], 1.5, 522],
  xtofrom: [["leftToFrom", "rightToFrom"], 1.75, 528],
  // The next three arrows are from the mhchem package.
  // In mhchem.sty, min-length is 2.0em. But these arrows might appear in the
  // document as \xrightarrow or \xrightleftharpoons. Those have
  // min-length = 1.75em, so we set min-length on these next three to match.
  xrightleftarrows: [["baraboveleftarrow", "rightarrowabovebar"], 1.75, 901],
  xrightequilibrium: [["baraboveshortleftharpoon", "rightharpoonaboveshortbar"], 1.75, 716],
  xleftequilibrium: [["shortbaraboveleftharpoon", "shortrightharpoonabovebar"], 1.75, 716]
}, yi = /* @__PURE__ */ new Set(["widehat", "widecheck", "widetilde", "utilde"]), Ze = function(e, t) {
  function a() {
    var o = 4e5, h = e.label.slice(1);
    if (yi.has(h) && "base" in e) {
      var m = e.base.type === "ordgroup" ? e.base.body.length : 1, v, b, k;
      if (m > 5)
        h === "widehat" || h === "widecheck" ? (v = 420, o = 2364, k = 0.42, b = h + "4") : (v = 312, o = 2340, k = 0.34, b = "tilde4");
      else {
        var S = [1, 1, 2, 2, 3, 3][m];
        h === "widehat" || h === "widecheck" ? (o = [0, 1062, 2364, 2364, 2364][S], v = [0, 239, 300, 360, 420][S], k = [0, 0.24, 0.3, 0.3, 0.36, 0.42][S], b = h + S) : (o = [0, 600, 1033, 2339, 2340][S], v = [0, 260, 286, 306, 312][S], k = [0, 0.26, 0.286, 0.3, 0.306, 0.34][S], b = "tilde" + S);
      }
      var _ = new ee(b), C = new G0([_], {
        width: "100%",
        height: B(k),
        viewBox: "0 0 " + o + " " + v,
        preserveAspectRatio: "none"
      });
      return {
        span: te([], [C], t),
        minWidth: 0,
        height: k
      };
    } else {
      var N = [], I = bi[h];
      if (!I)
        throw new Error('No SVG data for "' + h + '".');
      var [x, w, A] = I, T = A / 1e3, z = x.length, q, H;
      if (z === 1) {
        if (I.length !== 4)
          throw new Error('Expected 4-tuple for single-path SVG data "' + h + '".');
        q = ["hide-tail"], H = [I[3]];
      } else if (z === 2)
        q = ["halfarrow-left", "halfarrow-right"], H = ["xMinYMin", "xMaxYMin"];
      else if (z === 3)
        q = ["brace-left", "brace-center", "brace-right"], H = ["xMinYMin", "xMidYMin", "xMaxYMin"];
      else
        throw new Error(`Correct katexImagesData or update code here to support
                    ` + z + " children.");
      for (var R = 0; R < z; R++) {
        var X = new ee(x[R]), W = new G0([X], {
          width: "400em",
          height: B(T),
          viewBox: "0 0 " + o + " " + A,
          preserveAspectRatio: H[R] + " slice"
        }), Z = te([q[R]], [W], t);
        if (z === 1)
          return {
            span: Z,
            minWidth: w,
            height: T
          };
        Z.style.height = B(T), N.push(Z);
      }
      return {
        span: F(["stretchy"], N, t),
        minWidth: w,
        height: T
      };
    }
  }
  var {
    span: n,
    minWidth: s,
    height: u
  } = a();
  return n.height = u, n.style.height = B(u), s > 0 && (n.style.minWidth = B(s)), n;
}, xi = function(e, t, a, n, s) {
  var u, o = e.height + e.depth + a + n;
  if (/fbox|color|angl/.test(t)) {
    if (u = F(["stretchy", t], [], s), t === "fbox") {
      var h = s.color && s.getColor();
      h && (u.style.borderColor = h);
    }
  } else {
    var m = [];
    /^[bx]cancel$/.test(t) && m.push(new zt({
      x1: "0",
      y1: "0",
      x2: "100%",
      y2: "100%",
      "stroke-width": "0.046em"
    })), /^x?cancel$/.test(t) && m.push(new zt({
      x1: "0",
      y1: "100%",
      x2: "100%",
      y2: "0",
      "stroke-width": "0.046em"
    }));
    var v = new G0(m, {
      width: "100%",
      height: B(o)
    });
    u = te([], [v], s);
  }
  return u.height = o, u.style.height = B(o), u;
}, wi = {
  bin: 1,
  close: 1,
  inner: 1,
  open: 1,
  punct: 1,
  rel: 1
}, Si = {
  "accent-token": 1,
  mathord: 1,
  "op-token": 1,
  spacing: 1,
  textord: 1
};
function ki(r) {
  return r in wi;
}
function j(r, e) {
  if (!r || r.type !== e)
    throw new Error("Expected node of type " + e + ", but got " + (r ? "node of type " + r.type : String(r)));
  return r;
}
function Ke(r) {
  var e = Je(r);
  if (!e)
    throw new Error("Expected node of symbol group type, but got " + (r ? "node of type " + r.type : String(r)));
  return e;
}
function Je(r) {
  return r && (r.type === "atom" || Si.hasOwnProperty(r.type)) ? r : null;
}
var da = (r) => {
  if (r instanceof z0)
    return r;
  if (Wn(r) && r.children.length === 1)
    return da(r.children[0]);
}, ma = (r, e) => {
  var t, a, n;
  r && r.type === "supsub" ? (a = j(r.base, "accent"), t = a.base, r.base = t, n = Yn(Y(r, e)), r.base = a) : (a = j(r, "accent"), t = a.base);
  var s = Y(t, e.havingCrampedStyle()), u = a.isShifty && U0(t), o = 0;
  if (u) {
    var h, m;
    o = (h = (m = da(s)) == null ? void 0 : m.skew) != null ? h : 0;
  }
  var v = a.label === "\\c", b = v ? s.height + s.depth : Math.min(s.height, e.fontMetrics().xHeight), k;
  if (a.isStretchy)
    k = Ze(a, e), k = V({
      positionType: "firstBaseline",
      children: [{
        type: "elem",
        elem: s
      }, {
        type: "elem",
        elem: k,
        wrapperClasses: ["svg-align"],
        wrapperStyle: o > 0 ? {
          width: "calc(100% - " + B(2 * o) + ")",
          marginLeft: B(2 * o)
        } : void 0
      }]
    });
  else {
    var S, _;
    a.label === "\\vec" ? (S = ia("vec", e), _ = na.vec[1]) : (S = Ye({
      type: "textord",
      mode: a.mode,
      text: a.label
    }, e), S = Vn(S), S.italic = 0, _ = S.width, v && (b += S.depth)), k = F(["accent-body"], [S]);
    var C = a.label === "\\textcircled";
    C && (k.classes.push("accent-full"), b = s.height);
    var N = o;
    C || (N -= _ / 2), k.style.left = B(N), a.label === "\\textcircled" && (k.style.top = ".2em"), k = V({
      positionType: "firstBaseline",
      children: [{
        type: "elem",
        elem: s
      }, {
        type: "kern",
        size: -b
      }, {
        type: "elem",
        elem: k
      }]
    });
  }
  var I = F(["mord", "accent"], [k], e);
  return n ? (n.children[0] = I, n.height = Math.max(I.height, n.height), n.classes[0] = "mord", n) : I;
}, Ai = (r, e) => {
  var t = r.isStretchy ? We(r.label) : new D("mo", [C0(r.label, r.mode)]), a = new D("mover", [K(r.base, e), t]);
  return a.setAttribute("accent", "true"), a;
}, Ti = new RegExp(["\\acute", "\\grave", "\\ddot", "\\tilde", "\\bar", "\\breve", "\\check", "\\hat", "\\vec", "\\dot", "\\mathring"].map((r) => "\\" + r).join("|"));
O({
  type: "accent",
  names: ["\\acute", "\\grave", "\\ddot", "\\tilde", "\\bar", "\\breve", "\\check", "\\hat", "\\vec", "\\dot", "\\mathring", "\\widecheck", "\\widehat", "\\widetilde", "\\overrightarrow", "\\overleftarrow", "\\Overrightarrow", "\\overleftrightarrow", "\\overgroup", "\\overlinesegment", "\\overleftharpoon", "\\overrightharpoon"],
  numArgs: 1,
  handler: (r, e) => {
    var t = je(e[0]), a = !Ti.test(r.funcName), n = !a || r.funcName === "\\widehat" || r.funcName === "\\widetilde" || r.funcName === "\\widecheck";
    return {
      type: "accent",
      mode: r.parser.mode,
      label: r.funcName,
      isStretchy: a,
      isShifty: n,
      base: t
    };
  },
  htmlBuilder: ma,
  mathmlBuilder: Ai
});
O({
  type: "accent",
  names: ["\\'", "\\`", "\\^", "\\~", "\\=", "\\u", "\\.", '\\"', "\\c", "\\r", "\\H", "\\v", "\\textcircled"],
  numArgs: 1,
  allowedInText: !0,
  allowedInMath: !0,
  // unless in strict mode
  argTypes: ["primitive"],
  handler: (r, e) => {
    var t = e[0], a = r.parser.mode;
    return a === "math" && (r.parser.settings.reportNonstrict("mathVsTextAccents", "LaTeX's accent " + r.funcName + " works only in text mode"), a = "text"), {
      type: "accent",
      mode: a,
      label: r.funcName,
      isStretchy: !1,
      isShifty: !0,
      base: t
    };
  }
});
O({
  type: "accentUnder",
  names: ["\\underleftarrow", "\\underrightarrow", "\\underleftrightarrow", "\\undergroup", "\\underlinesegment", "\\utilde"],
  numArgs: 1,
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0];
    return {
      type: "accentUnder",
      mode: t.mode,
      label: a,
      base: n
    };
  },
  htmlBuilder: (r, e) => {
    var t = Y(r.base, e), a = Ze(r, e), n = r.label === "\\utilde" ? 0.12 : 0, s = V({
      positionType: "top",
      positionData: t.height,
      children: [{
        type: "elem",
        elem: a,
        wrapperClasses: ["svg-align"]
      }, {
        type: "kern",
        size: n
      }, {
        type: "elem",
        elem: t
      }]
    });
    return F(["mord", "accentunder"], [s], e);
  },
  mathmlBuilder: (r, e) => {
    var t = We(r.label), a = new D("munder", [K(r.base, e), t]);
    return a.setAttribute("accentunder", "true"), a;
  }
});
var Oe = (r) => {
  var e = new D("mpadded", r ? [r] : []);
  return e.setAttribute("width", "+0.6em"), e.setAttribute("lspace", "0.3em"), e;
};
O({
  type: "xArrow",
  names: [
    "\\xleftarrow",
    "\\xrightarrow",
    "\\xLeftarrow",
    "\\xRightarrow",
    "\\xleftrightarrow",
    "\\xLeftrightarrow",
    "\\xhookleftarrow",
    "\\xhookrightarrow",
    "\\xmapsto",
    "\\xrightharpoondown",
    "\\xrightharpoonup",
    "\\xleftharpoondown",
    "\\xleftharpoonup",
    "\\xrightleftharpoons",
    "\\xleftrightharpoons",
    "\\xlongequal",
    "\\xtwoheadrightarrow",
    "\\xtwoheadleftarrow",
    "\\xtofrom",
    // The next 3 functions are here to support the mhchem extension.
    // Direct use of these functions is discouraged and may break someday.
    "\\xrightleftarrows",
    "\\xrightequilibrium",
    "\\xleftequilibrium",
    // The next 3 functions are here only to support the {CD} environment.
    "\\\\cdrightarrow",
    "\\\\cdleftarrow",
    "\\\\cdlongequal"
  ],
  numArgs: 1,
  numOptionalArgs: 1,
  handler(r, e, t) {
    var {
      parser: a,
      funcName: n
    } = r;
    return {
      type: "xArrow",
      mode: a.mode,
      label: n,
      body: e[0],
      below: t[0]
    };
  },
  htmlBuilder(r, e) {
    var t = e.style, a = e.havingStyle(t.sup()), n = ve(Y(r.body, a, e), e), s = r.label.slice(0, 2) === "\\x" ? "x" : "cd";
    n.classes.push(s + "-arrow-pad");
    var u;
    r.below && (a = e.havingStyle(t.sub()), u = ve(Y(r.below, a, e), e), u.classes.push(s + "-arrow-pad"));
    var o = Ze(r, e), h = -e.fontMetrics().axisHeight + 0.5 * o.height, m = -e.fontMetrics().axisHeight - 0.5 * o.height - 0.111;
    (n.depth > 0.25 || r.label === "\\xleftequilibrium") && (m -= n.depth);
    var v;
    if (u) {
      var b = -e.fontMetrics().axisHeight + u.height + 0.5 * o.height + 0.111;
      v = V({
        positionType: "individualShift",
        children: [{
          type: "elem",
          elem: n,
          shift: m
        }, {
          type: "elem",
          elem: o,
          shift: h,
          wrapperClasses: ["svg-align"]
        }, {
          type: "elem",
          elem: u,
          shift: b
        }]
      });
    } else
      v = V({
        positionType: "individualShift",
        children: [{
          type: "elem",
          elem: n,
          shift: m
        }, {
          type: "elem",
          elem: o,
          shift: h,
          wrapperClasses: ["svg-align"]
        }]
      });
    return F(["mrel", "x-arrow"], [v], e);
  },
  mathmlBuilder(r, e) {
    var t = We(r.label);
    t.setAttribute("minsize", r.label.charAt(0) === "x" ? "1.75em" : "3.0em");
    var a;
    if (r.body) {
      var n = Oe(K(r.body, e));
      if (r.below) {
        var s = Oe(K(r.below, e));
        a = new D("munderover", [t, s, n]);
      } else
        a = new D("mover", [t, n]);
    } else if (r.below) {
      var u = Oe(K(r.below, e));
      a = new D("munder", [t, u]);
    } else
      a = Oe(), a = new D("mover", [t, a]);
    return a;
  }
});
function zi(r, e) {
  var t = o0(r.body, e, !0);
  return F([r.mclass], t, e);
}
function Mi(r, e) {
  var t, a = F0(r.body, e);
  return r.mclass === "minner" ? t = new D("mpadded", a) : r.mclass === "mord" ? r.isCharacterBox ? (t = a[0], t.type = "mi") : t = new D("mi", a) : (r.isCharacterBox ? (t = a[0], t.type = "mo") : t = new D("mo", a), r.mclass === "mbin" ? (t.attributes.lspace = "0.22em", t.attributes.rspace = "0.22em") : r.mclass === "mpunct" ? (t.attributes.lspace = "0em", t.attributes.rspace = "0.17em") : (r.mclass === "mopen" || r.mclass === "mclose") && (t.attributes.lspace = "0em", t.attributes.rspace = "0em")), t;
}
O({
  type: "mclass",
  names: ["\\mathord", "\\mathbin", "\\mathrel", "\\mathopen", "\\mathclose", "\\mathpunct", "\\mathinner"],
  numArgs: 1,
  primitive: !0,
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0];
    return {
      type: "mclass",
      mode: t.mode,
      mclass: "m" + a.slice(5),
      body: i0(n),
      isCharacterBox: U0(n)
    };
  },
  htmlBuilder: zi,
  mathmlBuilder: Mi
});
var Qe = (r) => {
  var e = r.type === "ordgroup" && r.body.length ? r.body[0] : r;
  return e.type === "atom" && (e.family === "bin" || e.family === "rel") ? "m" + e.family : "mord";
};
O({
  type: "mclass",
  names: ["\\@binrel"],
  numArgs: 2,
  handler(r, e) {
    var {
      parser: t
    } = r;
    return {
      type: "mclass",
      mode: t.mode,
      mclass: Qe(e[0]),
      body: i0(e[1]),
      isCharacterBox: U0(e[1])
    };
  }
});
O({
  type: "mclass",
  names: ["\\stackrel", "\\overset", "\\underset"],
  numArgs: 2,
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r, n = e[1], s = e[0], u;
    a !== "\\stackrel" ? u = Qe(n) : u = "mrel";
    var o = {
      type: "op",
      mode: n.mode,
      limits: !0,
      alwaysHandleSupSub: !0,
      parentIsSupSub: !1,
      symbol: !1,
      suppressBaseShift: a !== "\\stackrel",
      body: i0(n)
    }, h = a === "\\underset" ? {
      type: "supsub",
      mode: s.mode,
      base: o,
      sub: s
    } : {
      type: "supsub",
      mode: s.mode,
      base: o,
      sup: s
    };
    return {
      type: "mclass",
      mode: t.mode,
      mclass: u,
      body: [h],
      isCharacterBox: U0(h)
    };
  }
});
O({
  type: "pmb",
  names: ["\\pmb"],
  numArgs: 1,
  allowedInText: !0,
  handler(r, e) {
    var {
      parser: t
    } = r;
    return {
      type: "pmb",
      mode: t.mode,
      mclass: Qe(e[0]),
      body: i0(e[0])
    };
  },
  htmlBuilder(r, e) {
    var t = o0(r.body, e, !0), a = F([r.mclass], t, e);
    return a.style.textShadow = "0.02em 0.01em 0.04px", a;
  },
  mathmlBuilder(r, e) {
    var t = F0(r.body, e), a = new D("mstyle", t);
    return a.setAttribute("style", "text-shadow: 0.02em 0.01em 0.04px"), a;
  }
});
var Fi = {
  ">": "\\\\cdrightarrow",
  "<": "\\\\cdleftarrow",
  "=": "\\\\cdlongequal",
  A: "\\uparrow",
  V: "\\downarrow",
  "|": "\\Vert",
  ".": "no arrow"
}, Tr = () => ({
  type: "styling",
  body: [],
  mode: "math",
  style: "display",
  resetFont: !0
}), zr = (r) => r.type === "textord" && r.text === "@", Ni = (r, e) => (r.type === "mathord" || r.type === "atom") && r.text === e;
function Ci(r, e, t) {
  var a = Fi[r];
  switch (a) {
    case "\\\\cdrightarrow":
    case "\\\\cdleftarrow":
      return t.callFunction(a, [e[0]], [e[1]]);
    case "\\uparrow":
    case "\\downarrow": {
      var n = t.callFunction("\\\\cdleft", [e[0]], []), s = {
        type: "atom",
        text: a,
        mode: "math",
        family: "rel"
      }, u = t.callFunction("\\Big", [s], []), o = t.callFunction("\\\\cdright", [e[1]], []), h = {
        type: "ordgroup",
        mode: "math",
        body: [n, u, o]
      };
      return t.callFunction("\\\\cdparent", [h], []);
    }
    case "\\\\cdlongequal":
      return t.callFunction("\\\\cdlongequal", [], []);
    case "\\Vert": {
      var m = {
        type: "textord",
        text: "\\Vert",
        mode: "math"
      };
      return t.callFunction("\\Big", [m], []);
    }
    default:
      return {
        type: "textord",
        text: " ",
        mode: "math"
      };
  }
}
function Ei(r) {
  var e = [];
  for (r.gullet.beginGroup(), r.gullet.macros.set("\\cr", "\\\\\\relax"), r.gullet.beginGroup(); ; ) {
    e.push(r.parseExpression(!1, "\\\\")), r.gullet.endGroup(), r.gullet.beginGroup();
    var t = r.fetch().text;
    if (t === "&" || t === "\\\\")
      r.consume();
    else if (t === "\\end") {
      e[e.length - 1].length === 0 && e.pop();
      break;
    } else
      throw new E("Expected \\\\ or \\cr or \\end", r.nextToken);
  }
  for (var a = [], n = [a], s = 0; s < e.length; s++) {
    for (var u = e[s], o = Tr(), h = 0; h < u.length; h++)
      if (!zr(u[h]))
        o.body.push(u[h]);
      else {
        a.push(o), h += 1;
        var m = Ke(u[h]).text, v = new Array(2);
        if (v[0] = {
          type: "ordgroup",
          mode: "math",
          body: []
        }, v[1] = {
          type: "ordgroup",
          mode: "math",
          body: []
        }, !"=|.".includes(m)) if ("<>AV".includes(m))
          for (var b = 0; b < 2; b++) {
            for (var k = !0, S = h + 1; S < u.length; S++) {
              if (Ni(u[S], m)) {
                k = !1, h = S;
                break;
              }
              if (zr(u[S]))
                throw new E("Missing a " + m + " character to complete a CD arrow.", u[S]);
              v[b].body.push(u[S]);
            }
            if (k)
              throw new E("Missing a " + m + " character to complete a CD arrow.", u[h]);
          }
        else
          throw new E('Expected one of "<>AV=|." after @', u[h]);
        var _ = Ci(m, v, r), C = {
          type: "styling",
          body: [_],
          mode: "math",
          style: "display",
          // CD is always displaystyle.
          resetFont: !0
        };
        a.push(C), o = Tr();
      }
    s % 2 === 0 ? a.push(o) : a.shift(), a = [], n.push(a);
  }
  r.gullet.endGroup(), r.gullet.endGroup();
  var N = new Array(n[0].length).fill({
    type: "align",
    align: "c",
    pregap: 0.25,
    // CD package sets \enskip between columns.
    postgap: 0.25
    // So pre and post each get half an \enskip, i.e. 0.25em.
  });
  return {
    type: "array",
    mode: "math",
    body: n,
    arraystretch: 1,
    addJot: !0,
    rowGaps: [null],
    cols: N,
    colSeparationType: "CD",
    hLinesBeforeRow: new Array(n.length + 1).fill([])
  };
}
O({
  type: "cdlabel",
  names: ["\\\\cdleft", "\\\\cdright"],
  numArgs: 1,
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r;
    return {
      type: "cdlabel",
      mode: t.mode,
      side: a.slice(4),
      label: e[0]
    };
  },
  htmlBuilder(r, e) {
    var t = e.havingStyle(e.style.sup()), a = ve(Y(r.label, t, e), e);
    return a.classes.push("cd-label-" + r.side), a.style.bottom = B(0.8 - a.depth), a.height = 0, a.depth = 0, a;
  },
  mathmlBuilder(r, e) {
    var t = new D("mrow", [K(r.label, e)]);
    return t = new D("mpadded", [t]), t.setAttribute("width", "0"), r.side === "left" && t.setAttribute("lspace", "-1width"), t.setAttribute("voffset", "0.7em"), t = new D("mstyle", [t]), t.setAttribute("displaystyle", "false"), t.setAttribute("scriptlevel", "1"), t;
  }
});
O({
  type: "cdlabelparent",
  names: ["\\\\cdparent"],
  numArgs: 1,
  handler(r, e) {
    var {
      parser: t
    } = r;
    return {
      type: "cdlabelparent",
      mode: t.mode,
      fragment: e[0]
    };
  },
  htmlBuilder(r, e) {
    var t = ve(Y(r.fragment, e), e);
    return t.classes.push("cd-vert-arrow"), t;
  },
  mathmlBuilder(r, e) {
    return new D("mrow", [K(r.fragment, e)]);
  }
});
O({
  type: "textord",
  names: ["\\@char"],
  numArgs: 1,
  allowedInText: !0,
  handler(r, e) {
    for (var {
      parser: t
    } = r, a = j(e[0], "ordgroup"), n = a.body, s = "", u = 0; u < n.length; u++) {
      var o = j(n[u], "textord");
      s += o.text;
    }
    var h = parseInt(s), m;
    if (isNaN(h))
      throw new E("\\@char has non-numeric argument " + s);
    if (h < 0 || h >= 1114111)
      throw new E("\\@char with invalid code point " + s);
    return h <= 65535 ? m = String.fromCharCode(h) : (h -= 65536, m = String.fromCharCode((h >> 10) + 55296, (h & 1023) + 56320)), {
      type: "textord",
      mode: t.mode,
      text: m
    };
  }
});
var Di = (r, e) => {
  var t = o0(r.body, e.withColor(r.color), !1);
  return X0(t);
}, Bi = (r, e) => {
  var t = F0(r.body, e.withColor(r.color)), a = new D("mstyle", t);
  return a.setAttribute("mathcolor", r.color), a;
};
O({
  type: "color",
  names: ["\\textcolor"],
  numArgs: 2,
  allowedInText: !0,
  argTypes: ["color", "original"],
  handler(r, e) {
    var {
      parser: t
    } = r, a = j(e[0], "color-token").color, n = e[1];
    return {
      type: "color",
      mode: t.mode,
      color: a,
      body: i0(n)
    };
  },
  htmlBuilder: Di,
  mathmlBuilder: Bi
});
O({
  type: "color",
  names: ["\\color"],
  numArgs: 1,
  allowedInText: !0,
  argTypes: ["color"],
  handler(r, e) {
    var {
      parser: t,
      breakOnTokenText: a
    } = r, n = j(e[0], "color-token").color;
    t.gullet.macros.set("\\current@color", n);
    var s = t.parseExpression(!0, a);
    return {
      type: "color",
      mode: t.mode,
      color: n,
      body: s
    };
  }
});
O({
  type: "cr",
  names: ["\\\\"],
  numArgs: 0,
  numOptionalArgs: 0,
  allowedInText: !0,
  handler(r, e, t) {
    var {
      parser: a
    } = r, n = a.gullet.future().text === "[" ? a.parseSizeGroup(!0) : null, s = !a.settings.displayMode || !a.settings.useStrictBehavior("newLineInDisplayMode", "In LaTeX, \\\\ or \\newline does nothing in display mode");
    return {
      type: "cr",
      mode: a.mode,
      newLine: s,
      size: n && j(n, "size").value
    };
  },
  // The following builders are called only at the top level,
  // not within tabular/array environments.
  htmlBuilder(r, e) {
    var t = F(["mspace"], [], e);
    return r.newLine && (t.classes.push("newline"), r.size && (t.style.marginTop = B(a0(r.size, e)))), t;
  },
  mathmlBuilder(r, e) {
    var t = new D("mspace");
    return r.newLine && (t.setAttribute("linebreak", "newline"), r.size && t.setAttribute("height", B(a0(r.size, e)))), t;
  }
});
var _t = {
  "\\global": "\\global",
  "\\long": "\\\\globallong",
  "\\\\globallong": "\\\\globallong",
  "\\def": "\\gdef",
  "\\gdef": "\\gdef",
  "\\edef": "\\xdef",
  "\\xdef": "\\xdef",
  "\\let": "\\\\globallet",
  "\\futurelet": "\\\\globalfuture"
}, fa = (r) => {
  var e = r.text;
  if (/^(?:[\\{}$&#^_]|EOF)$/.test(e))
    throw new E("Expected a control sequence", r);
  return e;
}, Ii = (r) => {
  var e = r.gullet.popToken();
  return e.text === "=" && (e = r.gullet.popToken(), e.text === " " && (e = r.gullet.popToken())), e;
}, pa = (r, e, t, a) => {
  var n = r.gullet.macros.get(t.text);
  n == null && (t.noexpand = !0, n = {
    tokens: [t],
    numArgs: 0,
    // reproduce the same behavior in expansion
    unexpandable: !r.gullet.isExpandable(t.text)
  }), r.gullet.macros.set(e, n, a);
};
O({
  type: "internal",
  names: [
    "\\global",
    "\\long",
    "\\\\globallong"
    // can’t be entered directly
  ],
  numArgs: 0,
  allowedInText: !0,
  handler(r) {
    var {
      parser: e,
      funcName: t
    } = r;
    e.consumeSpaces();
    var a = e.fetch();
    if (_t[a.text])
      return (t === "\\global" || t === "\\\\globallong") && (a.text = _t[a.text]), j(e.parseFunction(), "internal");
    throw new E("Invalid token after macro prefix", a);
  }
});
O({
  type: "internal",
  names: ["\\def", "\\gdef", "\\edef", "\\xdef"],
  numArgs: 0,
  allowedInText: !0,
  primitive: !0,
  handler(r) {
    var {
      parser: e,
      funcName: t
    } = r, a = e.gullet.popToken(), n = a.text;
    if (/^(?:[\\{}$&#^_]|EOF)$/.test(n))
      throw new E("Expected a control sequence", a);
    for (var s = 0, u, o = [[]]; e.gullet.future().text !== "{"; )
      if (a = e.gullet.popToken(), a.text === "#") {
        if (e.gullet.future().text === "{") {
          u = e.gullet.future(), o[s].push("{");
          break;
        }
        if (a = e.gullet.popToken(), !/^[1-9]$/.test(a.text))
          throw new E('Invalid argument number "' + a.text + '"');
        if (parseInt(a.text) !== s + 1)
          throw new E('Argument number "' + a.text + '" out of order');
        s++, o.push([]);
      } else {
        if (a.text === "EOF")
          throw new E("Expected a macro definition");
        o[s].push(a.text);
      }
    var {
      tokens: h
    } = e.gullet.consumeArg();
    return u && h.unshift(u), (t === "\\edef" || t === "\\xdef") && (h = e.gullet.expandTokens(h), h.reverse()), e.gullet.macros.set(n, {
      tokens: h,
      numArgs: s,
      delimiters: o
    }, t === _t[t]), {
      type: "internal",
      mode: e.mode
    };
  }
});
O({
  type: "internal",
  names: [
    "\\let",
    "\\\\globallet"
    // can’t be entered directly
  ],
  numArgs: 0,
  allowedInText: !0,
  primitive: !0,
  handler(r) {
    var {
      parser: e,
      funcName: t
    } = r, a = fa(e.gullet.popToken());
    e.gullet.consumeSpaces();
    var n = Ii(e);
    return pa(e, a, n, t === "\\\\globallet"), {
      type: "internal",
      mode: e.mode
    };
  }
});
O({
  type: "internal",
  names: [
    "\\futurelet",
    "\\\\globalfuture"
    // can’t be entered directly
  ],
  numArgs: 0,
  allowedInText: !0,
  primitive: !0,
  handler(r) {
    var {
      parser: e,
      funcName: t
    } = r, a = fa(e.gullet.popToken()), n = e.gullet.popToken(), s = e.gullet.popToken();
    return pa(e, a, s, t === "\\\\globalfuture"), e.gullet.pushToken(s), e.gullet.pushToken(n), {
      type: "internal",
      mode: e.mode
    };
  }
});
var xe = function(e, t, a) {
  var n = Q.math[e] && Q.math[e].replace, s = Xt(n || e, t, a);
  if (!s)
    throw new Error("Unsupported symbol " + e + " and font size " + t + ".");
  return s;
}, Kt = function(e, t, a, n) {
  var s = a.havingBaseStyle(t), u = F(n.concat(s.sizingClasses(a)), [e], a), o = s.sizeMultiplier / a.sizeMultiplier;
  return u.height *= o, u.depth *= o, u.maxFontSize = s.sizeMultiplier, u;
}, va = function(e, t, a) {
  var n = t.havingBaseStyle(a), s = (1 - t.sizeMultiplier / n.sizeMultiplier) * t.fontMetrics().axisHeight;
  e.classes.push("delimcenter"), e.style.top = B(s), e.height -= s, e.depth += s;
}, Ri = function(e, t, a, n, s, u) {
  var o = p0(e, "Main-Regular", s, n), h = Kt(o, t, n, u);
  return va(h, n, t), h;
}, qi = function(e, t, a, n) {
  return p0(e, "Size" + t + "-Regular", a, n);
}, ga = function(e, t, a, n, s, u) {
  var o = qi(e, t, s, n), h = Kt(F(["delimsizing", "size" + t], [o], n), U.TEXT, n, u);
  return a && va(h, n, U.TEXT), h;
}, ht = function(e, t, a) {
  var n;
  t === "Size1-Regular" ? n = "delim-size1" : n = "delim-size4";
  var s = F(["delimsizinginner", n], [F([], [p0(e, t, a)])]);
  return {
    type: "elem",
    elem: s
  };
}, dt = function(e, t, a) {
  var n = I0["Size4-Regular"][e.charCodeAt(0)] ? I0["Size4-Regular"][e.charCodeAt(0)][4] : I0["Size1-Regular"][e.charCodeAt(0)][4], s = new ee("inner", Hn(e, Math.round(1e3 * t))), u = new G0([s], {
    width: B(n),
    height: B(t),
    // Override CSS rule `.katex svg { width: 100% }`
    style: "width:" + B(n),
    viewBox: "0 0 " + 1e3 * n + " " + Math.round(1e3 * t),
    preserveAspectRatio: "xMinYMin"
  }), o = te([], [u], a);
  return o.height = t, o.style.height = B(t), o.style.width = B(n), {
    type: "elem",
    elem: o
  };
}, Ot = 8e-3, Le = {
  type: "kern",
  size: -1 * Ot
}, _i = /* @__PURE__ */ new Set(["|", "\\lvert", "\\rvert", "\\vert"]), Oi = /* @__PURE__ */ new Set(["\\|", "\\lVert", "\\rVert", "\\Vert"]), ba = function(e, t, a, n, s, u) {
  var o, h, m, v, b = "", k = 0;
  o = m = v = e, h = null;
  var S = "Size1-Regular";
  e === "\\uparrow" ? m = v = "⏐" : e === "\\Uparrow" ? m = v = "‖" : e === "\\downarrow" ? o = m = "⏐" : e === "\\Downarrow" ? o = m = "‖" : e === "\\updownarrow" ? (o = "\\uparrow", m = "⏐", v = "\\downarrow") : e === "\\Updownarrow" ? (o = "\\Uparrow", m = "‖", v = "\\Downarrow") : _i.has(e) ? (m = "∣", b = "vert", k = 333) : Oi.has(e) ? (m = "∥", b = "doublevert", k = 556) : e === "[" || e === "\\lbrack" ? (o = "⎡", m = "⎢", v = "⎣", S = "Size4-Regular", b = "lbrack", k = 667) : e === "]" || e === "\\rbrack" ? (o = "⎤", m = "⎥", v = "⎦", S = "Size4-Regular", b = "rbrack", k = 667) : e === "\\lfloor" || e === "⌊" ? (m = o = "⎢", v = "⎣", S = "Size4-Regular", b = "lfloor", k = 667) : e === "\\lceil" || e === "⌈" ? (o = "⎡", m = v = "⎢", S = "Size4-Regular", b = "lceil", k = 667) : e === "\\rfloor" || e === "⌋" ? (m = o = "⎥", v = "⎦", S = "Size4-Regular", b = "rfloor", k = 667) : e === "\\rceil" || e === "⌉" ? (o = "⎤", m = v = "⎥", S = "Size4-Regular", b = "rceil", k = 667) : e === "(" || e === "\\lparen" ? (o = "⎛", m = "⎜", v = "⎝", S = "Size4-Regular", b = "lparen", k = 875) : e === ")" || e === "\\rparen" ? (o = "⎞", m = "⎟", v = "⎠", S = "Size4-Regular", b = "rparen", k = 875) : e === "\\{" || e === "\\lbrace" ? (o = "⎧", h = "⎨", v = "⎩", m = "⎪", S = "Size4-Regular") : e === "\\}" || e === "\\rbrace" ? (o = "⎫", h = "⎬", v = "⎭", m = "⎪", S = "Size4-Regular") : e === "\\lgroup" || e === "⟮" ? (o = "⎧", v = "⎩", m = "⎪", S = "Size4-Regular") : e === "\\rgroup" || e === "⟯" ? (o = "⎫", v = "⎭", m = "⎪", S = "Size4-Regular") : e === "\\lmoustache" || e === "⎰" ? (o = "⎧", v = "⎭", m = "⎪", S = "Size4-Regular") : (e === "\\rmoustache" || e === "⎱") && (o = "⎫", v = "⎩", m = "⎪", S = "Size4-Regular");
  var _ = xe(o, S, s), C = _.height + _.depth, N = xe(m, S, s), I = N.height + N.depth, x = xe(v, S, s), w = x.height + x.depth, A = 0, T = 1;
  if (h !== null) {
    var z = xe(h, S, s);
    A = z.height + z.depth, T = 2;
  }
  var q = C + w + A, H = Math.max(0, Math.ceil((t - q) / (T * I))), R = q + H * T * I, X = n.fontMetrics().axisHeight;
  a && (X *= n.sizeMultiplier);
  var W = R / 2 - X, Z = [];
  if (b.length > 0) {
    var V0 = R - C - w, x0 = Math.round(R * 1e3), w0 = $n(b, Math.round(V0 * 1e3)), D0 = new ee(b, w0), O0 = B(k / 1e3), t0 = B(x0 / 1e3), b0 = new G0([D0], {
      width: O0,
      height: t0,
      viewBox: "0 0 " + k + " " + x0
    }), m0 = te([], [b0], n);
    m0.height = x0 / 1e3, m0.style.width = O0, m0.style.height = t0, Z.push({
      type: "elem",
      elem: m0
    });
  } else {
    if (Z.push(ht(v, S, s)), Z.push(Le), h === null) {
      var S0 = R - C - w + 2 * Ot;
      Z.push(dt(m, S0, n));
    } else {
      var h0 = (R - C - w - A) / 2 + 2 * Ot;
      Z.push(dt(m, h0, n)), Z.push(Le), Z.push(ht(h, S, s)), Z.push(Le), Z.push(dt(m, h0, n));
    }
    Z.push(Le), Z.push(ht(o, S, s));
  }
  var k0 = n.havingBaseStyle(U.TEXT), A0 = V({
    positionType: "bottom",
    positionData: W,
    children: Z
  });
  return Kt(F(["delimsizing", "mult"], [A0], k0), U.TEXT, n, u);
}, mt = 80, ft = 0.08, pt = function(e, t, a, n, s) {
  var u = Ln(e, n, a), o = new ee(e, u), h = new G0([o], {
    // Note: 1000:1 ratio of viewBox to document em width.
    width: "400em",
    height: B(t),
    viewBox: "0 0 400000 " + a,
    preserveAspectRatio: "xMinYMin slice"
  });
  return te(["hide-tail"], [h], s);
}, Li = function(e, t) {
  var a = t.havingBaseSizing(), n = ka("\\surd", e * a.sizeMultiplier, Sa, a), s = a.sizeMultiplier, u = Math.max(0, t.minRuleThickness - t.fontMetrics().sqrtRuleThickness), o, h, m, v, b;
  return n.type === "small" ? (v = 1e3 + 1e3 * u + mt, e < 1 ? s = 1 : e < 1.4 && (s = 0.7), h = (1 + u + ft) / s, m = (1 + u) / s, o = pt("sqrtMain", h, v, u, t), o.style.minWidth = "0.853em", b = 0.833 / s) : n.type === "large" ? (v = (1e3 + mt) * we[n.size], m = (we[n.size] + u) / s, h = (we[n.size] + u + ft) / s, o = pt("sqrtSize" + n.size, h, v, u, t), o.style.minWidth = "1.02em", b = 1 / s) : (h = e + u + ft, m = e + u, v = Math.floor(1e3 * e + u) + mt, o = pt("sqrtTall", h, v, u, t), o.style.minWidth = "0.742em", b = 1.056), o.height = m, o.style.height = B(h), {
    span: o,
    advanceWidth: b,
    // Calculate the actual line width.
    // This actually should depend on the chosen font -- e.g. \boldmath
    // should use the thicker surd symbols from e.g. KaTeX_Main-Bold, and
    // have thicker rules.
    ruleWidth: (t.fontMetrics().sqrtRuleThickness + u) * s
  };
}, ya = /* @__PURE__ */ new Set(["(", "\\lparen", ")", "\\rparen", "[", "\\lbrack", "]", "\\rbrack", "\\{", "\\lbrace", "\\}", "\\rbrace", "\\lfloor", "\\rfloor", "⌊", "⌋", "\\lceil", "\\rceil", "⌈", "⌉", "\\surd"]), Hi = /* @__PURE__ */ new Set(["\\uparrow", "\\downarrow", "\\updownarrow", "\\Uparrow", "\\Downarrow", "\\Updownarrow", "|", "\\|", "\\vert", "\\Vert", "\\lvert", "\\rvert", "\\lVert", "\\rVert", "\\lgroup", "\\rgroup", "⟮", "⟯", "\\lmoustache", "\\rmoustache", "⎰", "⎱"]), xa = /* @__PURE__ */ new Set(["<", ">", "\\langle", "\\rangle", "/", "\\backslash", "\\lt", "\\gt"]), we = [0, 1.2, 1.8, 2.4, 3], wa = function(e, t, a, n, s) {
  if (e === "<" || e === "\\lt" || e === "⟨" ? e = "\\langle" : (e === ">" || e === "\\gt" || e === "⟩") && (e = "\\rangle"), ya.has(e) || xa.has(e))
    return ga(e, t, !1, a, n, s);
  if (Hi.has(e))
    return ba(e, we[t], !1, a, n, s);
  throw new E("Illegal delimiter: '" + e + "'");
}, $i = [{
  type: "small",
  style: U.SCRIPTSCRIPT
}, {
  type: "small",
  style: U.SCRIPT
}, {
  type: "small",
  style: U.TEXT
}, {
  type: "large",
  size: 1
}, {
  type: "large",
  size: 2
}, {
  type: "large",
  size: 3
}, {
  type: "large",
  size: 4
}], Pi = [{
  type: "small",
  style: U.SCRIPTSCRIPT
}, {
  type: "small",
  style: U.SCRIPT
}, {
  type: "small",
  style: U.TEXT
}, {
  type: "stack"
}], Sa = [{
  type: "small",
  style: U.SCRIPTSCRIPT
}, {
  type: "small",
  style: U.SCRIPT
}, {
  type: "small",
  style: U.TEXT
}, {
  type: "large",
  size: 1
}, {
  type: "large",
  size: 2
}, {
  type: "large",
  size: 3
}, {
  type: "large",
  size: 4
}, {
  type: "stack"
}], Gi = function(e) {
  if (e.type === "small")
    return "Main-Regular";
  if (e.type === "large")
    return "Size" + e.size + "-Regular";
  if (e.type === "stack")
    return "Size4-Regular";
  var t = e.type;
  throw new Error("Add support for delim type '" + t + "' here.");
}, ka = function(e, t, a, n) {
  for (var s = Math.min(2, 3 - n.style.size), u = s; u < a.length; u++) {
    var o = a[u];
    if (o.type === "stack")
      break;
    var h = xe(e, Gi(o), "math"), m = h.height + h.depth;
    if (o.type === "small") {
      var v = n.havingBaseStyle(o.style);
      m *= v.sizeMultiplier;
    }
    if (m > t)
      return o;
  }
  return a[a.length - 1];
}, Lt = function(e, t, a, n, s, u) {
  e === "<" || e === "\\lt" || e === "⟨" ? e = "\\langle" : (e === ">" || e === "\\gt" || e === "⟩") && (e = "\\rangle");
  var o;
  xa.has(e) ? o = $i : ya.has(e) ? o = Sa : o = Pi;
  var h = ka(e, t, o, n);
  return h.type === "small" ? Ri(e, h.style, a, n, s, u) : h.type === "large" ? ga(e, h.size, a, n, s, u) : ba(e, t, a, n, s, u);
}, vt = function(e, t, a, n, s, u) {
  var o = n.fontMetrics().axisHeight * n.sizeMultiplier, h = 901, m = 5 / n.fontMetrics().ptPerEm, v = Math.max(t - o, a + o), b = Math.max(
    // In real TeX, calculations are done using integral values which are
    // 65536 per pt, or 655360 per em. So, the division here truncates in
    // TeX but doesn't here, producing different results. If we wanted to
    // exactly match TeX's calculation, we could do
    //   Math.floor(655360 * maxDistFromAxis / 500) *
    //    delimiterFactor / 655360
    // (To see the difference, compare
    //    x^{x^{\left(\rule{0.1em}{0.68em}\right)}}
    // in TeX and KaTeX)
    v / 500 * h,
    2 * v - m
  );
  return Lt(e, b, !0, n, s, u);
}, Mr = {
  "\\bigl": {
    mclass: "mopen",
    size: 1
  },
  "\\Bigl": {
    mclass: "mopen",
    size: 2
  },
  "\\biggl": {
    mclass: "mopen",
    size: 3
  },
  "\\Biggl": {
    mclass: "mopen",
    size: 4
  },
  "\\bigr": {
    mclass: "mclose",
    size: 1
  },
  "\\Bigr": {
    mclass: "mclose",
    size: 2
  },
  "\\biggr": {
    mclass: "mclose",
    size: 3
  },
  "\\Biggr": {
    mclass: "mclose",
    size: 4
  },
  "\\bigm": {
    mclass: "mrel",
    size: 1
  },
  "\\Bigm": {
    mclass: "mrel",
    size: 2
  },
  "\\biggm": {
    mclass: "mrel",
    size: 3
  },
  "\\Biggm": {
    mclass: "mrel",
    size: 4
  },
  "\\big": {
    mclass: "mord",
    size: 1
  },
  "\\Big": {
    mclass: "mord",
    size: 2
  },
  "\\bigg": {
    mclass: "mord",
    size: 3
  },
  "\\Bigg": {
    mclass: "mord",
    size: 4
  }
}, Ui = /* @__PURE__ */ new Set(["(", "\\lparen", ")", "\\rparen", "[", "\\lbrack", "]", "\\rbrack", "\\{", "\\lbrace", "\\}", "\\rbrace", "\\lfloor", "\\rfloor", "⌊", "⌋", "\\lceil", "\\rceil", "⌈", "⌉", "<", ">", "\\langle", "⟨", "\\rangle", "⟩", "\\lt", "\\gt", "\\lvert", "\\rvert", "\\lVert", "\\rVert", "\\lgroup", "\\rgroup", "⟮", "⟯", "\\lmoustache", "\\rmoustache", "⎰", "⎱", "/", "\\backslash", "|", "\\vert", "\\|", "\\Vert", "\\uparrow", "\\Uparrow", "\\downarrow", "\\Downarrow", "\\updownarrow", "\\Updownarrow", "."]);
function Fr(r) {
  return "isMiddle" in r;
}
function et(r, e) {
  var t = Je(r);
  if (t && Ui.has(t.text))
    return t;
  throw t ? new E("Invalid delimiter '" + t.text + "' after '" + e.funcName + "'", r) : new E("Invalid delimiter type '" + r.type + "'", r);
}
O({
  type: "delimsizing",
  names: ["\\bigl", "\\Bigl", "\\biggl", "\\Biggl", "\\bigr", "\\Bigr", "\\biggr", "\\Biggr", "\\bigm", "\\Bigm", "\\biggm", "\\Biggm", "\\big", "\\Big", "\\bigg", "\\Bigg"],
  numArgs: 1,
  argTypes: ["primitive"],
  handler: (r, e) => {
    var t = et(e[0], r);
    return {
      type: "delimsizing",
      mode: r.parser.mode,
      size: Mr[r.funcName].size,
      mclass: Mr[r.funcName].mclass,
      delim: t.text
    };
  },
  htmlBuilder: (r, e) => r.delim === "." ? F([r.mclass]) : wa(r.delim, r.size, e, r.mode, [r.mclass]),
  mathmlBuilder: (r) => {
    var e = [];
    r.delim !== "." && e.push(C0(r.delim, r.mode));
    var t = new D("mo", e);
    r.mclass === "mopen" || r.mclass === "mclose" ? t.setAttribute("fence", "true") : t.setAttribute("fence", "false"), t.setAttribute("stretchy", "true");
    var a = B(we[r.size]);
    return t.setAttribute("minsize", a), t.setAttribute("maxsize", a), t;
  }
});
function Nr(r) {
  if (!r.body)
    throw new Error("Bug: The leftright ParseNode wasn't fully parsed.");
}
O({
  type: "leftright-right",
  names: ["\\right"],
  numArgs: 1,
  primitive: !0,
  handler: (r, e) => {
    var t = r.parser.gullet.macros.get("\\current@color");
    if (t && typeof t != "string")
      throw new E("\\current@color set to non-string in \\right");
    return {
      type: "leftright-right",
      mode: r.parser.mode,
      delim: et(e[0], r).text,
      color: t
      // undefined if not set via \color
    };
  }
});
O({
  type: "leftright",
  names: ["\\left"],
  numArgs: 1,
  primitive: !0,
  handler: (r, e) => {
    var t = et(e[0], r), a = r.parser;
    ++a.leftrightDepth;
    var n = a.parseExpression(!1);
    --a.leftrightDepth, a.expect("\\right", !1);
    var s = j(a.parseFunction(), "leftright-right");
    return {
      type: "leftright",
      mode: a.mode,
      body: n,
      left: t.text,
      right: s.delim,
      rightColor: s.color
    };
  },
  htmlBuilder: (r, e) => {
    Nr(r);
    for (var t = o0(r.body, e, !0, ["mopen", "mclose"]), a = 0, n = 0, s = !1, u = 0; u < t.length; u++) {
      var o = t[u];
      Fr(o) ? s = !0 : (a = Math.max(t[u].height, a), n = Math.max(t[u].depth, n));
    }
    a *= e.sizeMultiplier, n *= e.sizeMultiplier;
    var h;
    if (r.left === "." ? h = Te(e, ["mopen"]) : h = vt(r.left, a, n, e, r.mode, ["mopen"]), t.unshift(h), s)
      for (var m = 1; m < t.length; m++) {
        var v = t[m];
        if (Fr(v)) {
          var b = v.isMiddle;
          t[m] = vt(b.delim, a, n, b.options, r.mode, []);
        }
      }
    var k;
    if (r.right === ".")
      k = Te(e, ["mclose"]);
    else {
      var S = r.rightColor ? e.withColor(r.rightColor) : e;
      k = vt(r.right, a, n, S, r.mode, ["mclose"]);
    }
    return t.push(k), F(["minner"], t, e);
  },
  mathmlBuilder: (r, e) => {
    Nr(r);
    var t = F0(r.body, e);
    if (r.left !== ".") {
      var a = new D("mo", [C0(r.left, r.mode)]);
      a.setAttribute("fence", "true"), t.unshift(a);
    }
    if (r.right !== ".") {
      var n = new D("mo", [C0(r.right, r.mode)]);
      n.setAttribute("fence", "true"), r.rightColor && n.setAttribute("mathcolor", r.rightColor), t.push(n);
    }
    return Wt(t);
  }
});
O({
  type: "middle",
  names: ["\\middle"],
  numArgs: 1,
  primitive: !0,
  handler: (r, e) => {
    var t = et(e[0], r);
    if (!r.parser.leftrightDepth)
      throw new E("\\middle without preceding \\left", t);
    return {
      type: "middle",
      mode: r.parser.mode,
      delim: t.text
    };
  },
  htmlBuilder: (r, e) => {
    var t;
    return r.delim === "." ? t = Te(e, []) : (t = wa(r.delim, 1, e, r.mode, []), t.isMiddle = {
      delim: r.delim,
      options: e
    }), t;
  },
  mathmlBuilder: (r, e) => {
    var t = r.delim === "\\vert" || r.delim === "|" ? C0("|", "text") : C0(r.delim, r.mode), a = new D("mo", [t]);
    return a.setAttribute("fence", "true"), a.setAttribute("lspace", "0.05em"), a.setAttribute("rspace", "0.05em"), a;
  }
});
var ji = (r, e) => {
  var t = ve(Y(r.body, e), e), a = r.label.slice(1), n = e.sizeMultiplier, s, u, o = U0(r.body);
  if (a === "sout")
    s = F(["stretchy", "sout"]), s.height = e.fontMetrics().defaultRuleThickness / n, u = -0.5 * e.fontMetrics().xHeight;
  else if (a === "phase") {
    var h = a0({
      number: 0.6,
      unit: "pt"
    }, e), m = a0({
      number: 0.35,
      unit: "ex"
    }, e), v = e.havingBaseSizing();
    n = n / v.sizeMultiplier;
    var b = t.height + t.depth + h + m;
    t.style.paddingLeft = B(b / 2 + h);
    var k = Math.floor(1e3 * b * n), S = _n(k), _ = new G0([new ee("phase", S)], {
      width: "400em",
      height: B(k / 1e3),
      viewBox: "0 0 400000 " + k,
      preserveAspectRatio: "xMinYMin slice"
    });
    s = te(["hide-tail"], [_], e), s.style.height = B(b), u = t.depth + h + m;
  } else {
    /cancel/.test(a) ? o || t.classes.push("cancel-pad") : a === "angl" ? t.classes.push("anglpad") : t.classes.push("boxpad");
    var C, N, I = 0;
    /box/.test(a) ? (I = Math.max(
      e.fontMetrics().fboxrule,
      // default
      e.minRuleThickness
    ), C = e.fontMetrics().fboxsep + (a === "colorbox" ? 0 : I), N = C) : a === "angl" ? (I = Math.max(e.fontMetrics().defaultRuleThickness, e.minRuleThickness), C = 4 * I, N = Math.max(0, 0.25 - t.depth)) : (C = o ? 0.2 : 0, N = C), s = xi(t, a, C, N, e), /fbox|boxed|fcolorbox/.test(a) ? (s.style.borderStyle = "solid", s.style.borderWidth = B(I)) : a === "angl" && I !== 0.049 && (s.style.borderTopWidth = B(I), s.style.borderRightWidth = B(I)), u = t.depth + N, r.backgroundColor && (s.style.backgroundColor = r.backgroundColor, r.borderColor && (s.style.borderColor = r.borderColor));
  }
  var x;
  if (r.backgroundColor)
    x = V({
      positionType: "individualShift",
      children: [
        // Put the color background behind inner;
        {
          type: "elem",
          elem: s,
          shift: u
        },
        {
          type: "elem",
          elem: t,
          shift: 0
        }
      ]
    });
  else {
    var w = /cancel|phase/.test(a) ? ["svg-align"] : [];
    x = V({
      positionType: "individualShift",
      children: [
        // Write the \cancel stroke on top of inner.
        {
          type: "elem",
          elem: t,
          shift: 0
        },
        {
          type: "elem",
          elem: s,
          shift: u,
          wrapperClasses: w
        }
      ]
    });
  }
  return /cancel/.test(a) && (x.height = t.height, x.depth = t.depth), /cancel/.test(a) && !o ? F(["mord", "cancel-lap"], [x], e) : F(["mord"], [x], e);
}, Xi = (r, e) => {
  var t, a = new D(r.label.includes("colorbox") ? "mpadded" : "menclose", [K(r.body, e)]);
  switch (r.label) {
    case "\\cancel":
      a.setAttribute("notation", "updiagonalstrike");
      break;
    case "\\bcancel":
      a.setAttribute("notation", "downdiagonalstrike");
      break;
    case "\\phase":
      a.setAttribute("notation", "phasorangle");
      break;
    case "\\sout":
      a.setAttribute("notation", "horizontalstrike");
      break;
    case "\\fbox":
      a.setAttribute("notation", "box");
      break;
    case "\\angl":
      a.setAttribute("notation", "actuarial");
      break;
    case "\\fcolorbox":
    case "\\colorbox":
      if (t = e.fontMetrics().fboxsep * e.fontMetrics().ptPerEm, a.setAttribute("width", "+" + 2 * t + "pt"), a.setAttribute("height", "+" + 2 * t + "pt"), a.setAttribute("lspace", t + "pt"), a.setAttribute("voffset", t + "pt"), r.label === "\\fcolorbox") {
        var n = Math.max(
          e.fontMetrics().fboxrule,
          // default
          e.minRuleThickness
        );
        a.setAttribute("style", "border: " + B(n) + " solid " + r.borderColor);
      }
      break;
    case "\\xcancel":
      a.setAttribute("notation", "updiagonalstrike downdiagonalstrike");
      break;
  }
  return r.backgroundColor && a.setAttribute("mathbackground", r.backgroundColor), a;
};
O({
  type: "enclose",
  names: ["\\colorbox"],
  numArgs: 2,
  allowedInText: !0,
  argTypes: ["color", "hbox"],
  handler(r, e, t) {
    var {
      parser: a,
      funcName: n
    } = r, s = j(e[0], "color-token").color, u = e[1];
    return {
      type: "enclose",
      mode: a.mode,
      label: n,
      backgroundColor: s,
      body: u
    };
  },
  htmlBuilder: ji,
  mathmlBuilder: Xi
});
O({
  type: "enclose",
  names: ["\\fcolorbox"],
  numArgs: 3,
  allowedInText: !0,
  argTypes: ["color", "color", "hbox"],
  handler(r, e, t) {
    var {
      parser: a,
      funcName: n
    } = r, s = j(e[0], "color-token").color, u = j(e[1], "color-token").color, o = e[2];
    return {
      type: "enclose",
      mode: a.mode,
      label: n,
      backgroundColor: u,
      borderColor: s,
      body: o
    };
  }
});
O({
  type: "enclose",
  names: ["\\fbox"],
  numArgs: 1,
  argTypes: ["hbox"],
  allowedInText: !0,
  handler(r, e) {
    var {
      parser: t
    } = r;
    return {
      type: "enclose",
      mode: t.mode,
      label: "\\fbox",
      body: e[0]
    };
  }
});
O({
  type: "enclose",
  names: ["\\cancel", "\\bcancel", "\\xcancel", "\\phase"],
  numArgs: 1,
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0];
    return {
      type: "enclose",
      mode: t.mode,
      label: a,
      body: n
    };
  }
});
O({
  type: "enclose",
  names: ["\\sout"],
  numArgs: 1,
  allowedInText: !0,
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r;
    t.mode === "math" && t.settings.reportNonstrict("mathVsSout", "LaTeX's \\sout works only in text mode");
    var n = e[0];
    return {
      type: "enclose",
      mode: t.mode,
      label: a,
      body: n
    };
  }
});
O({
  type: "enclose",
  names: ["\\angl"],
  numArgs: 1,
  argTypes: ["hbox"],
  allowedInText: !1,
  handler(r, e) {
    var {
      parser: t
    } = r;
    return {
      type: "enclose",
      mode: t.mode,
      label: "\\angl",
      body: e[0]
    };
  }
});
var Aa = {};
function R0(r) {
  for (var {
    type: e,
    names: t,
    props: a,
    handler: n,
    htmlBuilder: s,
    mathmlBuilder: u
  } = r, o = {
    type: e,
    numArgs: a.numArgs || 0,
    allowedInText: !1,
    numOptionalArgs: 0,
    handler: n
  }, h = 0; h < t.length; ++h)
    Aa[t[h]] = o;
  s && (ke[e] = s), u && (Ae[e] = u);
}
var Ta = {};
function d(r, e) {
  Ta[r] = e;
}
class y0 {
  // End offset, zero-based exclusive.
  constructor(e, t, a) {
    this.lexer = void 0, this.start = void 0, this.end = void 0, this.lexer = e, this.start = t, this.end = a;
  }
  /**
   * Merges two `SourceLocation`s from location providers, given they are
   * provided in order of appearance.
   * - Returns the first one's location if only the first is provided.
   * - Returns a merged range of the first and the last if both are provided
   *   and their lexers match.
   * - Otherwise, returns null.
   */
  static range(e, t) {
    return t ? !e || !e.loc || !t.loc || e.loc.lexer !== t.loc.lexer ? null : new y0(e.loc.lexer, e.loc.start, t.loc.end) : e && e.loc;
  }
}
class T0 {
  // used in \noexpand
  constructor(e, t) {
    this.text = void 0, this.loc = void 0, this.noexpand = void 0, this.treatAsRelax = void 0, this.text = e, this.loc = t;
  }
  /**
   * Given a pair of tokens (this and endToken), compute a `Token` encompassing
   * the whole input range enclosed by these two.
   */
  range(e, t) {
    return new T0(t, y0.range(this, e));
  }
}
function Cr(r) {
  var e = [];
  r.consumeSpaces();
  var t = r.fetch().text;
  for (t === "\\relax" && (r.consume(), r.consumeSpaces(), t = r.fetch().text); t === "\\hline" || t === "\\hdashline"; )
    r.consume(), e.push(t === "\\hdashline"), r.consumeSpaces(), t = r.fetch().text;
  return e;
}
var tt = (r) => {
  var e = r.parser.settings;
  if (!e.displayMode)
    throw new E("{" + r.envName + "} can be used only in display mode.");
}, Vi = /* @__PURE__ */ new Set(["gather", "gather*"]);
function Jt(r) {
  if (!r.includes("ed"))
    return !r.includes("*");
}
function ae(r, e, t) {
  var {
    hskipBeforeAndAfter: a,
    addJot: n,
    cols: s,
    arraystretch: u,
    colSeparationType: o,
    autoTag: h,
    singleRow: m,
    emptySingleRow: v,
    maxNumCols: b,
    leqno: k
  } = e;
  if (r.gullet.beginGroup(), m || r.gullet.macros.set("\\cr", "\\\\\\relax"), !u) {
    var S = r.gullet.expandMacroAsText("\\arraystretch");
    if (S == null)
      u = 1;
    else if (u = parseFloat(S), !u || u < 0)
      throw new E("Invalid \\arraystretch: " + S);
  }
  r.gullet.beginGroup();
  var _ = [], C = [_], N = [], I = [], x = h != null ? [] : void 0;
  function w() {
    h && r.gullet.macros.set("\\@eqnsw", "1", !0);
  }
  function A() {
    x && (r.gullet.macros.get("\\df@tag") ? (x.push(r.subparse([new T0("\\df@tag")])), r.gullet.macros.set("\\df@tag", void 0, !0)) : x.push(!!h && r.gullet.macros.get("\\@eqnsw") === "1"));
  }
  for (w(), I.push(Cr(r)); ; ) {
    var T = r.parseExpression(!1, m ? "\\end" : "\\\\");
    r.gullet.endGroup(), r.gullet.beginGroup();
    var z = {
      type: "ordgroup",
      mode: r.mode,
      body: T
    };
    t && (z = {
      type: "styling",
      mode: r.mode,
      style: t,
      resetFont: !0,
      body: [z]
    }), _.push(z);
    var q = r.fetch().text;
    if (q === "&") {
      if (b && _.length === b) {
        if (m || o)
          throw new E("Too many tab characters: &", r.nextToken);
        r.settings.reportNonstrict("textEnv", "Too few columns specified in the {array} column argument.");
      }
      r.consume();
    } else if (q === "\\end") {
      A(), _.length === 1 && z.type === "styling" && z.body.length === 1 && z.body[0].type === "ordgroup" && z.body[0].body.length === 0 && (C.length > 1 || !v) && C.pop(), I.length < C.length + 1 && I.push([]);
      break;
    } else if (q === "\\\\") {
      r.consume();
      var H = void 0;
      r.gullet.future().text !== " " && (H = r.parseSizeGroup(!0)), N.push(H ? H.value : null), A(), I.push(Cr(r)), _ = [], C.push(_), w();
    } else
      throw new E("Expected & or \\\\ or \\cr or \\end", r.nextToken);
  }
  return r.gullet.endGroup(), r.gullet.endGroup(), {
    type: "array",
    mode: r.mode,
    addJot: n,
    arraystretch: u,
    body: C,
    cols: s,
    rowGaps: N,
    hskipBeforeAndAfter: a,
    hLinesBeforeRow: I,
    colSeparationType: o,
    tags: x,
    leqno: k
  };
}
function Qt(r) {
  return r.slice(0, 1) === "d" ? "display" : "text";
}
var q0 = function(e, t) {
  var a, n, s = e.body.length, u = e.hLinesBeforeRow, o = 0, h = new Array(s), m = [], v = Math.max(
    // From LaTeX \showthe\arrayrulewidth. Equals 0.04 em.
    t.fontMetrics().arrayRuleWidth,
    t.minRuleThickness
  ), b = 1 / t.fontMetrics().ptPerEm, k = 5 * b;
  if (e.colSeparationType && e.colSeparationType === "small") {
    var S = t.havingStyle(U.SCRIPT).sizeMultiplier;
    k = 0.2778 * (S / t.sizeMultiplier);
  }
  var _ = e.colSeparationType === "CD" ? a0({
    number: 3,
    unit: "ex"
  }, t) : 12 * b, C = 3 * b, N = e.arraystretch * _, I = 0.7 * N, x = 0.3 * N, w = 0;
  function A(Ee) {
    for (var De = 0; De < Ee.length; ++De)
      De > 0 && (w += 0.25), m.push({
        pos: w,
        isDashed: Ee[De]
      });
  }
  for (A(u[0]), a = 0; a < e.body.length; ++a) {
    var T = e.body[a], z = I, q = x;
    o < T.length && (o = T.length);
    var H = {
      cells: new Array(T.length),
      height: 0,
      depth: 0,
      pos: 0
    };
    for (n = 0; n < T.length; ++n) {
      var R = Y(T[n], t);
      q < R.depth && (q = R.depth), z < R.height && (z = R.height), H.cells[n] = R;
    }
    var X = e.rowGaps[a], W = 0;
    X && (W = a0(X, t), W > 0 && (W += x, q < W && (q = W), W = 0)), e.addJot && a < e.body.length - 1 && (q += C), H.height = z, H.depth = q, w += z, H.pos = w, w += q + W, h[a] = H, A(u[a + 1]);
  }
  var Z = w / 2 + t.fontMetrics().axisHeight, V0 = e.cols || [], x0 = [], w0, D0, O0 = [];
  if (e.tags && e.tags.some((Ee) => Ee))
    for (a = 0; a < s; ++a) {
      var t0 = h[a], b0 = t0.pos - Z, m0 = e.tags[a], S0 = void 0;
      m0 === !0 ? S0 = F(["eqn-num"], [], t) : m0 === !1 ? S0 = F([], [], t) : S0 = F([], o0(m0, t, !0), t), S0.depth = t0.depth, S0.height = t0.height, O0.push({
        type: "elem",
        elem: S0,
        shift: b0
      });
    }
  for (
    n = 0, D0 = 0;
    // Continue while either there are more columns or more column
    // descriptions, so trailing separators don't get lost.
    n < o || D0 < V0.length;
    ++n, ++D0
  ) {
    for (var h0, k0 = V0[D0], A0 = !0; ((Y0 = k0) == null ? void 0 : Y0.type) === "separator"; ) {
      var Y0;
      if (A0 || (w0 = F(["arraycolsep"], []), w0.style.width = B(t.fontMetrics().doubleRuleSep), x0.push(w0)), k0.separator === "|" || k0.separator === ":") {
        var oe = k0.separator === "|" ? "solid" : "dashed", E0 = F(["vertical-separator"], [], t);
        E0.style.height = B(w), E0.style.borderRightWidth = B(v), E0.style.borderRightStyle = oe, E0.style.margin = "0 " + B(-v / 2);
        var ce = w - Z;
        ce && (E0.style.verticalAlign = B(-ce)), x0.push(E0);
      } else
        throw new E("Invalid separator type: " + k0.separator);
      D0++, k0 = V0[D0], A0 = !1;
    }
    if (!(n >= o)) {
      var L0 = void 0;
      if (n > 0 || e.hskipBeforeAndAfter) {
        var he, ne;
        L0 = (he = (ne = k0) == null ? void 0 : ne.pregap) != null ? he : k, L0 !== 0 && (w0 = F(["arraycolsep"], []), w0.style.width = B(L0), x0.push(w0));
      }
      var Fe = [];
      for (a = 0; a < s; ++a) {
        var W0 = h[a], Ne = W0.cells[n];
        if (Ne) {
          var $a = W0.pos - Z;
          Ne.depth = W0.depth, Ne.height = W0.height, Fe.push({
            type: "elem",
            elem: Ne,
            shift: $a
          });
        }
      }
      var Pa = V({
        positionType: "individualShift",
        children: Fe
      }), Ga = F(["col-align-" + (((h0 = k0) == null ? void 0 : h0.align) || "c")], [Pa]);
      if (x0.push(Ga), n < o - 1 || e.hskipBeforeAndAfter) {
        var nr, ir;
        L0 = (nr = (ir = k0) == null ? void 0 : ir.postgap) != null ? nr : k, L0 !== 0 && (w0 = F(["arraycolsep"], []), w0.style.width = B(L0), x0.push(w0));
      }
    }
  }
  var Ce = F(["mtable"], x0);
  if (m.length > 0) {
    for (var Ua = pe("hline", t, v), ja = pe("hdashline", t, v), at = [{
      type: "elem",
      elem: Ce,
      shift: 0
    }]; m.length > 0; ) {
      var sr = m.pop(), lr = sr.pos - Z;
      sr.isDashed ? at.push({
        type: "elem",
        elem: ja,
        shift: lr
      }) : at.push({
        type: "elem",
        elem: Ua,
        shift: lr
      });
    }
    Ce = V({
      positionType: "individualShift",
      children: at
    });
  }
  if (O0.length === 0)
    return F(["mord"], [Ce], t);
  var Xa = V({
    positionType: "individualShift",
    children: O0
  }), Va = F(["tag"], [Xa], t);
  return X0([Ce, Va]);
}, Yi = {
  c: "center ",
  l: "left ",
  r: "right "
}, _0 = function(e, t) {
  for (var a = [], n = new D("mtd", [], ["mtr-glue"]), s = new D("mtd", [], ["mml-eqn-num"]), u = 0; u < e.body.length; u++) {
    for (var o = e.body[u], h = [], m = 0; m < o.length; m++)
      h.push(new D("mtd", [K(o[m], t)]));
    e.tags && e.tags[u] && (h.unshift(n), h.push(n), e.leqno ? h.unshift(s) : h.push(s)), a.push(new D("mtr", h));
  }
  var v = new D("mtable", a), b = e.arraystretch === 0.5 ? 0.1 : 0.16 + e.arraystretch - 1 + (e.addJot ? 0.09 : 0);
  v.setAttribute("rowspacing", B(b));
  var k = "", S = "";
  if (e.cols && e.cols.length > 0) {
    var _ = e.cols, C = "", N = !1, I = 0, x = _.length;
    _[0].type === "separator" && (k += "top ", I = 1), _[_.length - 1].type === "separator" && (k += "bottom ", x -= 1);
    for (var w = I; w < x; w++) {
      var A = _[w];
      A.type === "align" ? (S += Yi[A.align], N && (C += "none "), N = !0) : A.type === "separator" && N && (C += A.separator === "|" ? "solid " : "dashed ", N = !1);
    }
    v.setAttribute("columnalign", S.trim()), /[sd]/.test(C) && v.setAttribute("columnlines", C.trim());
  }
  if (e.colSeparationType === "align") {
    for (var T = e.cols || [], z = "", q = 1; q < T.length; q++)
      z += q % 2 ? "0em " : "1em ";
    v.setAttribute("columnspacing", z.trim());
  } else e.colSeparationType === "alignat" || e.colSeparationType === "gather" ? v.setAttribute("columnspacing", "0em") : e.colSeparationType === "small" ? v.setAttribute("columnspacing", "0.2778em") : e.colSeparationType === "CD" ? v.setAttribute("columnspacing", "0.5em") : v.setAttribute("columnspacing", "1em");
  var H = "", R = e.hLinesBeforeRow;
  k += R[0].length > 0 ? "left " : "", k += R[R.length - 1].length > 0 ? "right " : "";
  for (var X = 1; X < R.length - 1; X++)
    H += R[X].length === 0 ? "none " : R[X][0] ? "dashed " : "solid ";
  return /[sd]/.test(H) && v.setAttribute("rowlines", H.trim()), k !== "" && (v = new D("menclose", [v]), v.setAttribute("notation", k.trim())), e.arraystretch && e.arraystretch < 1 && (v = new D("mstyle", [v]), v.setAttribute("scriptlevel", "1")), v;
}, za = function(e, t) {
  e.envName.includes("ed") || tt(e);
  var a = [], n = e.envName === "split", s = ae(e.parser, {
    cols: a,
    addJot: !0,
    autoTag: n ? void 0 : Jt(e.envName),
    emptySingleRow: !0,
    colSeparationType: e.envName.includes("at") ? "alignat" : "align",
    maxNumCols: n ? 2 : void 0,
    leqno: e.parser.settings.leqno
  }, "display"), u = 0, o = 0, h = {
    type: "ordgroup",
    mode: e.mode,
    body: []
  };
  if (t[0] && t[0].type === "ordgroup") {
    for (var m = "", v = 0; v < t[0].body.length; v++) {
      var b = j(t[0].body[v], "textord");
      m += b.text;
    }
    u = Number(m), o = u * 2;
  }
  var k = !o;
  s.body.forEach(function(N) {
    for (var I = 1; I < N.length; I += 2) {
      var x = j(N[I], "styling"), w = j(x.body[0], "ordgroup");
      w.body.unshift(h);
    }
    if (k)
      o < N.length && (o = N.length);
    else {
      var A = N.length / 2;
      if (u < A)
        throw new E("Too many math in a row: " + ("expected " + u + ", but got " + A), N[0]);
    }
  });
  for (var S = 0; S < o; ++S) {
    var _ = "r", C = 0;
    S % 2 === 1 ? _ = "l" : S > 0 && k && (C = 1), a[S] = {
      type: "align",
      align: _,
      pregap: C,
      postgap: 0
    };
  }
  return s.colSeparationType = k ? "align" : "alignat", s;
};
R0({
  type: "array",
  names: ["array", "darray"],
  props: {
    numArgs: 1
  },
  handler(r, e) {
    var t = Je(e[0]), a = t ? [e[0]] : j(e[0], "ordgroup").body, n = a.map(function(u) {
      var o = Ke(u), h = o.text;
      if ("lcr".includes(h))
        return {
          type: "align",
          align: h
        };
      if (h === "|")
        return {
          type: "separator",
          separator: "|"
        };
      if (h === ":")
        return {
          type: "separator",
          separator: ":"
        };
      throw new E("Unknown column alignment: " + h, u);
    }), s = {
      cols: n,
      hskipBeforeAndAfter: !0,
      // \@preamble in lttab.dtx
      maxNumCols: n.length
    };
    return ae(r.parser, s, Qt(r.envName));
  },
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["matrix", "pmatrix", "bmatrix", "Bmatrix", "vmatrix", "Vmatrix", "matrix*", "pmatrix*", "bmatrix*", "Bmatrix*", "vmatrix*", "Vmatrix*"],
  props: {
    numArgs: 0
  },
  handler(r) {
    var e = {
      matrix: null,
      pmatrix: ["(", ")"],
      bmatrix: ["[", "]"],
      Bmatrix: ["\\{", "\\}"],
      vmatrix: ["|", "|"],
      Vmatrix: ["\\Vert", "\\Vert"]
    }[r.envName.replace("*", "")], t = "c", a = {
      hskipBeforeAndAfter: !1,
      cols: [{
        type: "align",
        align: t
      }]
    };
    if (r.envName.charAt(r.envName.length - 1) === "*") {
      var n = r.parser;
      if (n.consumeSpaces(), n.fetch().text === "[") {
        if (n.consume(), n.consumeSpaces(), t = n.fetch().text, !"lcr".includes(t))
          throw new E("Expected l or c or r", n.nextToken);
        n.consume(), n.consumeSpaces(), n.expect("]"), n.consume(), a.cols = [{
          type: "align",
          align: t
        }];
      }
    }
    var s = ae(r.parser, a, Qt(r.envName)), u = Math.max(0, ...s.body.map((o) => o.length));
    return s.cols = new Array(u).fill({
      type: "align",
      align: t
    }), e ? {
      type: "leftright",
      mode: r.mode,
      body: [s],
      left: e[0],
      right: e[1],
      rightColor: void 0
      // \right uninfluenced by \color in array
    } : s;
  },
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["smallmatrix"],
  props: {
    numArgs: 0
  },
  handler(r) {
    var e = {
      arraystretch: 0.5
    }, t = ae(r.parser, e, "script");
    return t.colSeparationType = "small", t;
  },
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["subarray"],
  props: {
    numArgs: 1
  },
  handler(r, e) {
    var t = Je(e[0]), a = t ? [e[0]] : j(e[0], "ordgroup").body, n = a.map(function(o) {
      var h = Ke(o), m = h.text;
      if ("lc".includes(m))
        return {
          type: "align",
          align: m
        };
      throw new E("Unknown column alignment: " + m, o);
    });
    if (n.length > 1)
      throw new E("{subarray} can contain only one column");
    var s = {
      cols: n,
      hskipBeforeAndAfter: !1,
      arraystretch: 0.5
    }, u = ae(r.parser, s, "script");
    if (u.body.length > 0 && u.body[0].length > 1)
      throw new E("{subarray} can contain only one column");
    return u;
  },
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["cases", "dcases", "rcases", "drcases"],
  props: {
    numArgs: 0
  },
  handler(r) {
    var e = {
      arraystretch: 1.2,
      cols: [{
        type: "align",
        align: "l",
        pregap: 0,
        // TODO(kevinb) get the current style.
        // For now we use the metrics for TEXT style which is what we were
        // doing before.  Before attempting to get the current style we
        // should look at TeX's behavior especially for \over and matrices.
        postgap: 1
        /* 1em quad */
      }, {
        type: "align",
        align: "l",
        pregap: 0,
        postgap: 0
      }]
    }, t = ae(r.parser, e, Qt(r.envName));
    return {
      type: "leftright",
      mode: r.mode,
      body: [t],
      left: r.envName.includes("r") ? "." : "\\{",
      right: r.envName.includes("r") ? "\\}" : ".",
      rightColor: void 0
    };
  },
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["align", "align*", "aligned", "split"],
  props: {
    numArgs: 0
  },
  handler: za,
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["gathered", "gather", "gather*"],
  props: {
    numArgs: 0
  },
  handler(r) {
    Vi.has(r.envName) && tt(r);
    var e = {
      cols: [{
        type: "align",
        align: "c"
      }],
      addJot: !0,
      colSeparationType: "gather",
      autoTag: Jt(r.envName),
      emptySingleRow: !0,
      leqno: r.parser.settings.leqno
    };
    return ae(r.parser, e, "display");
  },
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["alignat", "alignat*", "alignedat"],
  props: {
    numArgs: 1
  },
  handler: za,
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["equation", "equation*"],
  props: {
    numArgs: 0
  },
  handler(r) {
    tt(r);
    var e = {
      autoTag: Jt(r.envName),
      emptySingleRow: !0,
      singleRow: !0,
      maxNumCols: 1,
      leqno: r.parser.settings.leqno
    };
    return ae(r.parser, e, "display");
  },
  htmlBuilder: q0,
  mathmlBuilder: _0
});
R0({
  type: "array",
  names: ["CD"],
  props: {
    numArgs: 0
  },
  handler(r) {
    return tt(r), Ei(r.parser);
  },
  htmlBuilder: q0,
  mathmlBuilder: _0
});
d("\\nonumber", "\\gdef\\@eqnsw{0}");
d("\\notag", "\\nonumber");
O({
  type: "text",
  // Doesn't matter what this is.
  names: ["\\hline", "\\hdashline"],
  numArgs: 0,
  allowedInText: !0,
  allowedInMath: !0,
  handler(r, e) {
    throw new E(r.funcName + " valid only within array environment");
  }
});
var Er = Aa;
O({
  type: "environment",
  names: ["\\begin", "\\end"],
  numArgs: 1,
  argTypes: ["text"],
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0];
    if (n.type !== "ordgroup")
      throw new E("Invalid environment name", n);
    for (var s = "", u = 0; u < n.body.length; ++u)
      s += j(n.body[u], "textord").text;
    if (a === "\\begin") {
      if (!Er.hasOwnProperty(s))
        throw new E("No such environment: " + s, n);
      var o = Er[s], {
        args: h,
        optArgs: m
      } = t.parseArguments("\\begin{" + s + "}", o), v = {
        mode: t.mode,
        envName: s,
        parser: t
      }, b = o.handler(v, h, m);
      t.expect("\\end", !1);
      var k = t.nextToken, S = j(t.parseFunction(), "environment");
      if (S.name !== s)
        throw new E("Mismatch: \\begin{" + s + "} matched by \\end{" + S.name + "}", k);
      return b;
    }
    return {
      type: "environment",
      mode: t.mode,
      name: s,
      nameGroup: n
    };
  }
});
var Wi = (r, e) => {
  var t = r.font, a = e.withFont(t);
  return Y(r.body, a);
}, Zi = (r, e) => {
  var t = r.font, a = e.withFont(t);
  return K(r.body, a);
}, Dr = {
  "\\Bbb": "\\mathbb",
  "\\bold": "\\mathbf",
  "\\frak": "\\mathfrak"
};
O({
  type: "font",
  names: [
    // styles, except \boldsymbol defined below
    "\\mathrm",
    "\\mathit",
    "\\mathbf",
    "\\mathnormal",
    "\\mathsfit",
    // families
    "\\mathbb",
    "\\mathcal",
    "\\mathfrak",
    "\\mathscr",
    "\\mathsf",
    "\\mathtt",
    // aliases, except \bm defined below
    "\\Bbb",
    "\\bold",
    "\\frak"
  ],
  numArgs: 1,
  allowedInArgument: !0,
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a
    } = r, n = je(e[0]), s = a in Dr ? Dr[a] : a;
    return {
      type: "font",
      mode: t.mode,
      font: s.slice(1),
      body: n
    };
  },
  htmlBuilder: Wi,
  mathmlBuilder: Zi
});
O({
  type: "mclass",
  names: ["\\boldsymbol", "\\bm"],
  numArgs: 1,
  handler: (r, e) => {
    var {
      parser: t
    } = r, a = e[0];
    return {
      type: "mclass",
      mode: t.mode,
      mclass: Qe(a),
      body: [{
        type: "font",
        mode: t.mode,
        font: "boldsymbol",
        body: a
      }],
      isCharacterBox: U0(a)
    };
  }
});
O({
  type: "font",
  names: ["\\rm", "\\sf", "\\tt", "\\bf", "\\it", "\\cal"],
  numArgs: 0,
  allowedInText: !0,
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a,
      breakOnTokenText: n
    } = r, {
      mode: s
    } = t, u = t.parseExpression(!0, n);
    return {
      type: "font",
      mode: s,
      font: "math" + a.slice(1),
      body: {
        type: "ordgroup",
        mode: t.mode,
        body: u
      }
    };
  }
});
var Ki = (r, e) => {
  var t = e.style, a = t.fracNum(), n = t.fracDen(), s;
  s = e.havingStyle(a);
  var u = Y(r.numer, s, e);
  if (r.continued) {
    var o = 8.5 / e.fontMetrics().ptPerEm, h = 3.5 / e.fontMetrics().ptPerEm;
    u.height = u.height < o ? o : u.height, u.depth = u.depth < h ? h : u.depth;
  }
  s = e.havingStyle(n);
  var m = Y(r.denom, s, e), v, b, k;
  r.hasBarLine ? (r.barSize ? (b = a0(r.barSize, e), v = pe("frac-line", e, b)) : v = pe("frac-line", e), b = v.height, k = v.height) : (v = null, b = 0, k = e.fontMetrics().defaultRuleThickness);
  var S, _, C;
  t.size === U.DISPLAY.size ? (S = e.fontMetrics().num1, b > 0 ? _ = 3 * k : _ = 7 * k, C = e.fontMetrics().denom1) : (b > 0 ? (S = e.fontMetrics().num2, _ = k) : (S = e.fontMetrics().num3, _ = 3 * k), C = e.fontMetrics().denom2);
  var N;
  if (v) {
    var x = e.fontMetrics().axisHeight;
    S - u.depth - (x + 0.5 * b) < _ && (S += _ - (S - u.depth - (x + 0.5 * b))), x - 0.5 * b - (m.height - C) < _ && (C += _ - (x - 0.5 * b - (m.height - C)));
    var w = -(x - 0.5 * b);
    N = V({
      positionType: "individualShift",
      children: [{
        type: "elem",
        elem: m,
        shift: C
      }, {
        type: "elem",
        elem: v,
        shift: w
      }, {
        type: "elem",
        elem: u,
        shift: -S
      }]
    });
  } else {
    var I = S - u.depth - (m.height - C);
    I < _ && (S += 0.5 * (_ - I), C += 0.5 * (_ - I)), N = V({
      positionType: "individualShift",
      children: [{
        type: "elem",
        elem: m,
        shift: C
      }, {
        type: "elem",
        elem: u,
        shift: -S
      }]
    });
  }
  s = e.havingStyle(t), N.height *= s.sizeMultiplier / e.sizeMultiplier, N.depth *= s.sizeMultiplier / e.sizeMultiplier;
  var A;
  t.size === U.DISPLAY.size ? A = e.fontMetrics().delim1 : t.size === U.SCRIPTSCRIPT.size ? A = e.havingStyle(U.SCRIPT).fontMetrics().delim2 : A = e.fontMetrics().delim2;
  var T, z;
  return r.leftDelim == null ? T = Te(e, ["mopen"]) : T = Lt(r.leftDelim, A, !0, e.havingStyle(t), r.mode, ["mopen"]), r.continued ? z = F([]) : r.rightDelim == null ? z = Te(e, ["mclose"]) : z = Lt(r.rightDelim, A, !0, e.havingStyle(t), r.mode, ["mclose"]), F(["mord"].concat(s.sizingClasses(e)), [T, F(["mfrac"], [N]), z], e);
}, Ji = (r, e) => {
  var t = new D("mfrac", [K(r.numer, e), K(r.denom, e)]);
  if (!r.hasBarLine)
    t.setAttribute("linethickness", "0px");
  else if (r.barSize) {
    var a = a0(r.barSize, e);
    t.setAttribute("linethickness", B(a));
  }
  if (r.leftDelim != null || r.rightDelim != null) {
    var n = [];
    if (r.leftDelim != null) {
      var s = new D("mo", [new s0(r.leftDelim.replace("\\", ""))]);
      s.setAttribute("fence", "true"), n.push(s);
    }
    if (n.push(t), r.rightDelim != null) {
      var u = new D("mo", [new s0(r.rightDelim.replace("\\", ""))]);
      u.setAttribute("fence", "true"), n.push(u);
    }
    return Wt(n);
  }
  return t;
}, Ma = (r, e) => {
  if (!e)
    return r;
  var t = {
    type: "styling",
    mode: r.mode,
    style: e,
    body: [r]
  };
  return t;
};
O({
  type: "genfrac",
  names: [
    "\\cfrac",
    "\\dfrac",
    "\\frac",
    "\\tfrac",
    "\\dbinom",
    "\\binom",
    "\\tbinom",
    "\\\\atopfrac",
    // can’t be entered directly
    "\\\\bracefrac",
    "\\\\brackfrac"
    // ditto
  ],
  numArgs: 2,
  allowedInArgument: !0,
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0], s = e[1], u, o = null, h = null;
    switch (a) {
      case "\\cfrac":
      case "\\dfrac":
      case "\\frac":
      case "\\tfrac":
        u = !0;
        break;
      case "\\\\atopfrac":
        u = !1;
        break;
      case "\\dbinom":
      case "\\binom":
      case "\\tbinom":
        u = !1, o = "(", h = ")";
        break;
      case "\\\\bracefrac":
        u = !1, o = "\\{", h = "\\}";
        break;
      case "\\\\brackfrac":
        u = !1, o = "[", h = "]";
        break;
      default:
        throw new Error("Unrecognized genfrac command");
    }
    var m = a === "\\cfrac", v = null;
    return m || a.startsWith("\\d") ? v = "display" : a.startsWith("\\t") && (v = "text"), Ma({
      type: "genfrac",
      mode: t.mode,
      numer: n,
      denom: s,
      continued: m,
      hasBarLine: u,
      leftDelim: o,
      rightDelim: h,
      barSize: null
    }, v);
  },
  htmlBuilder: Ki,
  mathmlBuilder: Ji
});
O({
  type: "infix",
  names: ["\\over", "\\choose", "\\atop", "\\brace", "\\brack"],
  numArgs: 0,
  infix: !0,
  handler(r) {
    var {
      parser: e,
      funcName: t,
      token: a
    } = r, n;
    switch (t) {
      case "\\over":
        n = "\\frac";
        break;
      case "\\choose":
        n = "\\binom";
        break;
      case "\\atop":
        n = "\\\\atopfrac";
        break;
      case "\\brace":
        n = "\\\\bracefrac";
        break;
      case "\\brack":
        n = "\\\\brackfrac";
        break;
      default:
        throw new Error("Unrecognized infix genfrac command");
    }
    return {
      type: "infix",
      mode: e.mode,
      replaceWith: n,
      token: a
    };
  }
});
var Br = ["display", "text", "script", "scriptscript"], Ir = function(e) {
  var t = null;
  return e.length > 0 && (t = e, t = t === "." ? null : t), t;
};
O({
  type: "genfrac",
  names: ["\\genfrac"],
  numArgs: 6,
  allowedInArgument: !0,
  argTypes: ["math", "math", "size", "text", "math", "math"],
  handler(r, e) {
    var {
      parser: t
    } = r, a = e[4], n = e[5], s = je(e[0]), u = s.type === "atom" && s.family === "open" ? Ir(s.text) : null, o = je(e[1]), h = o.type === "atom" && o.family === "close" ? Ir(o.text) : null, m = j(e[2], "size"), v, b = null;
    m.isBlank ? v = !0 : (b = m.value, v = b.number > 0);
    var k = null, S = e[3];
    if (S.type === "ordgroup") {
      if (S.body.length > 0) {
        var _ = j(S.body[0], "textord");
        k = Br[Number(_.text)];
      }
    } else
      S = j(S, "textord"), k = Br[Number(S.text)];
    return Ma({
      type: "genfrac",
      mode: t.mode,
      numer: a,
      denom: n,
      continued: !1,
      hasBarLine: v,
      barSize: b,
      leftDelim: u,
      rightDelim: h
    }, k);
  }
});
O({
  type: "infix",
  names: ["\\above"],
  numArgs: 1,
  argTypes: ["size"],
  infix: !0,
  handler(r, e) {
    var {
      parser: t,
      funcName: a,
      token: n
    } = r;
    return {
      type: "infix",
      mode: t.mode,
      replaceWith: "\\\\abovefrac",
      size: j(e[0], "size").value,
      token: n
    };
  }
});
O({
  type: "genfrac",
  names: ["\\\\abovefrac"],
  numArgs: 3,
  argTypes: ["math", "size", "math"],
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0], s = j(e[1], "infix").size;
    if (!s)
      throw new Error("\\\\abovefrac expected size, but got " + String(s));
    var u = e[2], o = s.number > 0;
    return {
      type: "genfrac",
      mode: t.mode,
      numer: n,
      denom: u,
      continued: !1,
      hasBarLine: o,
      barSize: s,
      leftDelim: null,
      rightDelim: null
    };
  }
});
var Fa = (r, e) => {
  var t = e.style, a, n;
  r.type === "supsub" ? (a = r.sup ? Y(r.sup, e.havingStyle(t.sup()), e) : Y(r.sub, e.havingStyle(t.sub()), e), n = j(r.base, "horizBrace")) : n = j(r, "horizBrace");
  var s = Y(n.base, e.havingBaseStyle(U.DISPLAY)), u = Ze(n, e), o;
  if (n.isOver ? o = V({
    positionType: "firstBaseline",
    children: [{
      type: "elem",
      elem: s
    }, {
      type: "kern",
      size: 0.1
    }, {
      type: "elem",
      elem: u,
      wrapperClasses: ["svg-align"]
    }]
  }) : o = V({
    positionType: "bottom",
    positionData: s.depth + 0.1 + u.height,
    children: [{
      type: "elem",
      elem: u,
      wrapperClasses: ["svg-align"]
    }, {
      type: "kern",
      size: 0.1
    }, {
      type: "elem",
      elem: s
    }]
  }), a) {
    var h = F(["minner", n.isOver ? "mover" : "munder"], [o], e);
    n.isOver ? o = V({
      positionType: "firstBaseline",
      children: [{
        type: "elem",
        elem: h
      }, {
        type: "kern",
        size: 0.2
      }, {
        type: "elem",
        elem: a
      }]
    }) : o = V({
      positionType: "bottom",
      positionData: h.depth + 0.2 + a.height + a.depth,
      children: [{
        type: "elem",
        elem: a
      }, {
        type: "kern",
        size: 0.2
      }, {
        type: "elem",
        elem: h
      }]
    });
  }
  return F(["minner", n.isOver ? "mover" : "munder"], [o], e);
}, Qi = (r, e) => {
  var t = We(r.label);
  return new D(r.isOver ? "mover" : "munder", [K(r.base, e), t]);
};
O({
  type: "horizBrace",
  names: ["\\overbrace", "\\underbrace", "\\overbracket", "\\underbracket"],
  numArgs: 1,
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r;
    return {
      type: "horizBrace",
      mode: t.mode,
      label: a,
      isOver: a.includes("\\over"),
      base: e[0]
    };
  },
  htmlBuilder: Fa,
  mathmlBuilder: Qi
});
O({
  type: "href",
  names: ["\\href"],
  numArgs: 2,
  argTypes: ["url", "original"],
  allowedInText: !0,
  handler: (r, e) => {
    var {
      parser: t
    } = r, a = e[1], n = j(e[0], "url").url;
    return t.settings.isTrusted({
      command: "\\href",
      url: n
    }) ? {
      type: "href",
      mode: t.mode,
      href: n,
      body: i0(a)
    } : t.formatUnsupportedCmd("\\href");
  },
  htmlBuilder: (r, e) => {
    var t = o0(r.body, e, !1);
    return ai(r.href, [], t, e);
  },
  mathmlBuilder: (r, e) => {
    var t = re(r.body, e);
    return t instanceof D || (t = new D("mrow", [t])), t.setAttribute("href", r.href), t;
  }
});
O({
  type: "href",
  names: ["\\url"],
  numArgs: 1,
  argTypes: ["url"],
  allowedInText: !0,
  handler: (r, e) => {
    var {
      parser: t
    } = r, a = j(e[0], "url").url;
    if (!t.settings.isTrusted({
      command: "\\url",
      url: a
    }))
      return t.formatUnsupportedCmd("\\url");
    for (var n = [], s = 0; s < a.length; s++) {
      var u = a[s];
      u === "~" && (u = "\\textasciitilde"), n.push({
        type: "textord",
        mode: "text",
        text: u
      });
    }
    var o = {
      type: "text",
      mode: t.mode,
      font: "\\texttt",
      body: n
    };
    return {
      type: "href",
      mode: t.mode,
      href: a,
      body: i0(o)
    };
  }
});
O({
  type: "hbox",
  names: ["\\hbox"],
  numArgs: 1,
  argTypes: ["text"],
  allowedInText: !0,
  primitive: !0,
  handler(r, e) {
    var {
      parser: t
    } = r;
    return {
      type: "hbox",
      mode: t.mode,
      body: i0(e[0])
    };
  },
  htmlBuilder(r, e) {
    var t = o0(r.body, e.withFont(""), !1);
    return X0(t);
  },
  mathmlBuilder(r, e) {
    return new D("mrow", F0(r.body, e.withFont("")));
  }
});
O({
  type: "html",
  names: ["\\htmlClass", "\\htmlId", "\\htmlStyle", "\\htmlData"],
  numArgs: 2,
  argTypes: ["raw", "original"],
  allowedInText: !0,
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a,
      token: n
    } = r, s = j(e[0], "raw").string, u = e[1];
    t.settings.strict && t.settings.reportNonstrict("htmlExtension", "HTML extension is disabled on strict mode");
    var o, h = {};
    switch (a) {
      case "\\htmlClass":
        h.class = s, o = {
          command: "\\htmlClass",
          class: s
        };
        break;
      case "\\htmlId":
        h.id = s, o = {
          command: "\\htmlId",
          id: s
        };
        break;
      case "\\htmlStyle":
        h.style = s, o = {
          command: "\\htmlStyle",
          style: s
        };
        break;
      case "\\htmlData": {
        for (var m = s.split(","), v = 0; v < m.length; v++) {
          var b = m[v], k = b.indexOf("=");
          if (k < 0)
            throw new E("\\htmlData key/value '" + b + "' missing equals sign");
          var S = b.slice(0, k), _ = b.slice(k + 1);
          h["data-" + S.trim()] = _;
        }
        o = {
          command: "\\htmlData",
          attributes: h
        };
        break;
      }
      default:
        throw new Error("Unrecognized html command");
    }
    return t.settings.isTrusted(o) ? {
      type: "html",
      mode: t.mode,
      attributes: h,
      body: i0(u)
    } : t.formatUnsupportedCmd(a);
  },
  htmlBuilder: (r, e) => {
    var t = o0(r.body, e, !1), a = ["enclosing"];
    r.attributes.class && a.push(...r.attributes.class.trim().split(/\s+/));
    var n = F(a, t, e);
    for (var s in r.attributes)
      s !== "class" && r.attributes.hasOwnProperty(s) && n.setAttribute(s, r.attributes[s]);
    return n;
  },
  mathmlBuilder: (r, e) => re(r.body, e)
});
O({
  type: "htmlmathml",
  names: ["\\html@mathml"],
  numArgs: 2,
  allowedInArgument: !0,
  allowedInText: !0,
  handler: (r, e) => {
    var {
      parser: t
    } = r;
    return {
      type: "htmlmathml",
      mode: t.mode,
      html: i0(e[0]),
      mathml: i0(e[1])
    };
  },
  htmlBuilder: (r, e) => {
    var t = o0(r.html, e, !1);
    return X0(t);
  },
  mathmlBuilder: (r, e) => re(r.mathml, e)
});
var gt = function(e) {
  if (/^[-+]? *(\d+(\.\d*)?|\.\d+)$/.test(e))
    return {
      number: +e,
      unit: "bp"
    };
  var t = /([-+]?) *(\d+(?:\.\d*)?|\.\d+) *([a-z]{2})/.exec(e);
  if (!t)
    throw new E("Invalid size: '" + e + "' in \\includegraphics");
  var a = {
    number: +(t[1] + t[2]),
    // sign + magnitude, cast to number
    unit: t[3]
  };
  if (!Kr(a))
    throw new E("Invalid unit: '" + a.unit + "' in \\includegraphics.");
  return a;
};
O({
  type: "includegraphics",
  names: ["\\includegraphics"],
  numArgs: 1,
  numOptionalArgs: 1,
  argTypes: ["raw", "url"],
  allowedInText: !1,
  handler: (r, e, t) => {
    var {
      parser: a
    } = r, n = {
      number: 0,
      unit: "em"
    }, s = {
      number: 0.9,
      unit: "em"
    }, u = {
      number: 0,
      unit: "em"
    }, o = "";
    if (t[0])
      for (var h = j(t[0], "raw").string, m = h.split(","), v = 0; v < m.length; v++) {
        var b = m[v].split("=");
        if (b.length === 2) {
          var k = b[1].trim();
          switch (b[0].trim()) {
            case "alt":
              o = k;
              break;
            case "width":
              n = gt(k);
              break;
            case "height":
              s = gt(k);
              break;
            case "totalheight":
              u = gt(k);
              break;
            default:
              throw new E("Invalid key: '" + b[0] + "' in \\includegraphics.");
          }
        }
      }
    var S = j(e[0], "url").url;
    return o === "" && (o = S, o = o.replace(/^.*[\\/]/, ""), o = o.substring(0, o.lastIndexOf("."))), a.settings.isTrusted({
      command: "\\includegraphics",
      url: S
    }) ? {
      type: "includegraphics",
      mode: a.mode,
      alt: o,
      width: n,
      height: s,
      totalheight: u,
      src: S
    } : a.formatUnsupportedCmd("\\includegraphics");
  },
  htmlBuilder: (r, e) => {
    var t = a0(r.height, e), a = 0;
    r.totalheight.number > 0 && (a = a0(r.totalheight, e) - t);
    var n = 0;
    r.width.number > 0 && (n = a0(r.width, e));
    var s = {
      height: B(t + a)
    };
    n > 0 && (s.width = B(n)), a > 0 && (s.verticalAlign = B(-a));
    var u = new jn(r.src, r.alt, s);
    return u.height = t, u.depth = a, u;
  },
  mathmlBuilder: (r, e) => {
    var t = new D("mglyph", []);
    t.setAttribute("alt", r.alt);
    var a = a0(r.height, e), n = 0;
    if (r.totalheight.number > 0 && (n = a0(r.totalheight, e) - a, t.setAttribute("valign", B(-n))), t.setAttribute("height", B(a + n)), r.width.number > 0) {
      var s = a0(r.width, e);
      t.setAttribute("width", B(s));
    }
    return t.setAttribute("src", r.src), t;
  }
});
O({
  type: "kern",
  names: ["\\kern", "\\mkern", "\\hskip", "\\mskip"],
  numArgs: 1,
  argTypes: ["size"],
  primitive: !0,
  allowedInText: !0,
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r, n = j(e[0], "size");
    if (t.settings.strict) {
      var s = a[1] === "m", u = n.value.unit === "mu";
      s ? (u || t.settings.reportNonstrict("mathVsTextUnits", "LaTeX's " + a + " supports only mu units, " + ("not " + n.value.unit + " units")), t.mode !== "math" && t.settings.reportNonstrict("mathVsTextUnits", "LaTeX's " + a + " works only in math mode")) : u && t.settings.reportNonstrict("mathVsTextUnits", "LaTeX's " + a + " doesn't support mu units");
    }
    return {
      type: "kern",
      mode: t.mode,
      dimension: n.value
    };
  },
  htmlBuilder(r, e) {
    return aa(r.dimension, e);
  },
  mathmlBuilder(r, e) {
    var t = a0(r.dimension, e);
    return new oa(t);
  }
});
O({
  type: "lap",
  names: ["\\mathllap", "\\mathrlap", "\\mathclap"],
  numArgs: 1,
  allowedInText: !0,
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0];
    return {
      type: "lap",
      mode: t.mode,
      alignment: a.slice(5),
      body: n
    };
  },
  htmlBuilder: (r, e) => {
    var t;
    r.alignment === "clap" ? (t = F([], [Y(r.body, e)]), t = F(["inner"], [t], e)) : t = F(["inner"], [Y(r.body, e)]);
    var a = F(["fix"], []), n = F([r.alignment], [t, a], e), s = F(["strut"]);
    return s.style.height = B(n.height + n.depth), n.depth && (s.style.verticalAlign = B(-n.depth)), n.children.unshift(s), n = F(["thinbox"], [n], e), F(["mord", "vbox"], [n], e);
  },
  mathmlBuilder: (r, e) => {
    var t = new D("mpadded", [K(r.body, e)]);
    if (r.alignment !== "rlap") {
      var a = r.alignment === "llap" ? "-1" : "-0.5";
      t.setAttribute("lspace", a + "width");
    }
    return t.setAttribute("width", "0px"), t;
  }
});
O({
  type: "styling",
  names: ["\\(", "$"],
  numArgs: 0,
  allowedInText: !0,
  allowedInMath: !1,
  handler(r, e) {
    var {
      funcName: t,
      parser: a
    } = r, n = a.mode;
    a.switchMode("math");
    var s = t === "\\(" ? "\\)" : "$", u = a.parseExpression(!1, s);
    return a.expect(s), a.switchMode(n), {
      type: "styling",
      mode: a.mode,
      style: "text",
      resetFont: !0,
      body: u
    };
  }
});
O({
  type: "text",
  // Doesn't matter what this is.
  names: ["\\)", "\\]"],
  numArgs: 0,
  allowedInText: !0,
  allowedInMath: !1,
  handler(r, e) {
    throw new E("Mismatched " + r.funcName);
  }
});
var Rr = (r, e) => {
  switch (e.style.size) {
    case U.DISPLAY.size:
      return r.display;
    case U.TEXT.size:
      return r.text;
    case U.SCRIPT.size:
      return r.script;
    case U.SCRIPTSCRIPT.size:
      return r.scriptscript;
    default:
      return r.text;
  }
};
O({
  type: "mathchoice",
  names: ["\\mathchoice"],
  numArgs: 4,
  primitive: !0,
  handler: (r, e) => {
    var {
      parser: t
    } = r;
    return {
      type: "mathchoice",
      mode: t.mode,
      display: i0(e[0]),
      text: i0(e[1]),
      script: i0(e[2]),
      scriptscript: i0(e[3])
    };
  },
  htmlBuilder: (r, e) => {
    var t = Rr(r, e), a = o0(t, e, !1);
    return X0(a);
  },
  mathmlBuilder: (r, e) => {
    var t = Rr(r, e);
    return re(t, e);
  }
});
var Na = (r, e, t, a, n, s, u) => {
  r = F([], [r]);
  var o = t && U0(t), h, m;
  if (e) {
    var v = Y(e, a.havingStyle(n.sup()), a);
    m = {
      elem: v,
      kern: Math.max(a.fontMetrics().bigOpSpacing1, a.fontMetrics().bigOpSpacing3 - v.depth)
    };
  }
  if (t) {
    var b = Y(t, a.havingStyle(n.sub()), a);
    h = {
      elem: b,
      kern: Math.max(a.fontMetrics().bigOpSpacing2, a.fontMetrics().bigOpSpacing4 - b.height)
    };
  }
  var k;
  if (m && h) {
    var S = a.fontMetrics().bigOpSpacing5 + h.elem.height + h.elem.depth + h.kern + r.depth + u;
    k = V({
      positionType: "bottom",
      positionData: S,
      children: [{
        type: "kern",
        size: a.fontMetrics().bigOpSpacing5
      }, {
        type: "elem",
        elem: h.elem,
        marginLeft: B(-s)
      }, {
        type: "kern",
        size: h.kern
      }, {
        type: "elem",
        elem: r
      }, {
        type: "kern",
        size: m.kern
      }, {
        type: "elem",
        elem: m.elem,
        marginLeft: B(s)
      }, {
        type: "kern",
        size: a.fontMetrics().bigOpSpacing5
      }]
    });
  } else if (h) {
    var _ = r.height - u;
    k = V({
      positionType: "top",
      positionData: _,
      children: [{
        type: "kern",
        size: a.fontMetrics().bigOpSpacing5
      }, {
        type: "elem",
        elem: h.elem,
        marginLeft: B(-s)
      }, {
        type: "kern",
        size: h.kern
      }, {
        type: "elem",
        elem: r
      }]
    });
  } else if (m) {
    var C = r.depth + u;
    k = V({
      positionType: "bottom",
      positionData: C,
      children: [{
        type: "elem",
        elem: r
      }, {
        type: "kern",
        size: m.kern
      }, {
        type: "elem",
        elem: m.elem,
        marginLeft: B(s)
      }, {
        type: "kern",
        size: a.fontMetrics().bigOpSpacing5
      }]
    });
  } else
    return r;
  var N = [k];
  if (h && s !== 0 && !o) {
    var I = F(["mspace"], [], a);
    I.style.marginRight = B(s), N.unshift(I);
  }
  return F(["mop", "op-limits"], N, a);
}, Ca = /* @__PURE__ */ new Set(["\\smallint"]), Ea = (r, e) => {
  var t, a, n = !1, s;
  r.type === "supsub" ? (t = r.sup, a = r.sub, s = j(r.base, "op"), n = !0) : s = j(r, "op");
  var u = e.style, o = !1;
  u.size === U.DISPLAY.size && s.symbol && !Ca.has(s.name) && (o = !0);
  var h, m;
  if (s.symbol) {
    var v = o ? "Size2-Regular" : "Size1-Regular", b = "";
    if ((s.name === "\\oiint" || s.name === "\\oiiint") && (b = s.name.slice(1), s.name = b === "oiint" ? "\\iint" : "\\iiint"), h = p0(s.name, v, "math", e, ["mop", "op-symbol", o ? "large-op" : "small-op"]), m = h.italic, b.length > 0) {
      var k = ia(b + "Size" + (o ? "2" : "1"), e);
      h = V({
        positionType: "individualShift",
        children: [{
          type: "elem",
          elem: h,
          shift: 0
        }, {
          type: "elem",
          elem: k,
          shift: o ? 0.08 : 0
        }]
      }), s.name = "\\" + b, h.classes.unshift("mop"), h.italic = m;
    }
  } else if (s.body) {
    var S = o0(s.body, e, !0);
    S.length === 1 && S[0] instanceof z0 ? (h = S[0], h.classes[0] = "mop") : h = F(["mop"], S, e);
  } else {
    for (var _ = [], C = 1; C < s.name.length; C++)
      _.push(Vt(s.name[C], s.mode, e));
    h = F(["mop"], _, e);
  }
  var N = 0, I = 0;
  if ((h instanceof z0 || s.name === "\\oiint" || s.name === "\\oiiint") && !s.suppressBaseShift) {
    var x;
    N = (h.height - h.depth) / 2 - e.fontMetrics().axisHeight, I = (x = h.italic) != null ? x : 0;
  }
  return n ? Na(h, t, a, e, u, I, N) : (N && (h.style.position = "relative", h.style.top = B(N)), h);
}, es = (r, e) => {
  var t;
  if (r.symbol)
    t = new D("mo", [C0(r.name, r.mode)]), Ca.has(r.name) && t.setAttribute("largeop", "false");
  else if (r.body)
    t = new D("mo", F0(r.body, e));
  else {
    t = new D("mi", [new s0(r.name.slice(1))]);
    var a = new D("mo", [C0("⁡", "text")]);
    r.parentIsSupSub ? t = new D("mrow", [t, a]) : t = ua([t, a]);
  }
  return t;
}, ts = {
  "∏": "\\prod",
  "∐": "\\coprod",
  "∑": "\\sum",
  "⋀": "\\bigwedge",
  "⋁": "\\bigvee",
  "⋂": "\\bigcap",
  "⋃": "\\bigcup",
  "⨀": "\\bigodot",
  "⨁": "\\bigoplus",
  "⨂": "\\bigotimes",
  "⨄": "\\biguplus",
  "⨆": "\\bigsqcup"
};
O({
  type: "op",
  names: ["\\coprod", "\\bigvee", "\\bigwedge", "\\biguplus", "\\bigcap", "\\bigcup", "\\intop", "\\prod", "\\sum", "\\bigotimes", "\\bigoplus", "\\bigodot", "\\bigsqcup", "\\smallint", "∏", "∐", "∑", "⋀", "⋁", "⋂", "⋃", "⨀", "⨁", "⨂", "⨄", "⨆"],
  numArgs: 0,
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a
    } = r, n = a;
    return n.length === 1 && (n = ts[n]), {
      type: "op",
      mode: t.mode,
      limits: !0,
      parentIsSupSub: !1,
      symbol: !0,
      name: n
    };
  },
  htmlBuilder: Ea,
  mathmlBuilder: es
});
O({
  type: "op",
  names: ["\\mathop"],
  numArgs: 1,
  primitive: !0,
  handler: (r, e) => {
    var {
      parser: t
    } = r, a = e[0];
    return {
      type: "op",
      mode: t.mode,
      limits: !1,
      parentIsSupSub: !1,
      symbol: !1,
      body: i0(a)
    };
  }
});
var rs = {
  "∫": "\\int",
  "∬": "\\iint",
  "∭": "\\iiint",
  "∮": "\\oint",
  "∯": "\\oiint",
  "∰": "\\oiiint"
};
O({
  type: "op",
  names: ["\\arcsin", "\\arccos", "\\arctan", "\\arctg", "\\arcctg", "\\arg", "\\ch", "\\cos", "\\cosec", "\\cosh", "\\cot", "\\cotg", "\\coth", "\\csc", "\\ctg", "\\cth", "\\deg", "\\dim", "\\exp", "\\hom", "\\ker", "\\lg", "\\ln", "\\log", "\\sec", "\\sin", "\\sinh", "\\sh", "\\tan", "\\tanh", "\\tg", "\\th"],
  numArgs: 0,
  handler(r) {
    var {
      parser: e,
      funcName: t
    } = r;
    return {
      type: "op",
      mode: e.mode,
      limits: !1,
      parentIsSupSub: !1,
      symbol: !1,
      name: t
    };
  }
});
O({
  type: "op",
  names: ["\\det", "\\gcd", "\\inf", "\\lim", "\\max", "\\min", "\\Pr", "\\sup"],
  numArgs: 0,
  handler(r) {
    var {
      parser: e,
      funcName: t
    } = r;
    return {
      type: "op",
      mode: e.mode,
      limits: !0,
      parentIsSupSub: !1,
      symbol: !1,
      name: t
    };
  }
});
O({
  type: "op",
  names: ["\\int", "\\iint", "\\iiint", "\\oint", "\\oiint", "\\oiiint", "∫", "∬", "∭", "∮", "∯", "∰"],
  numArgs: 0,
  allowedInArgument: !0,
  handler(r) {
    var {
      parser: e,
      funcName: t
    } = r, a = t;
    return a.length === 1 && (a = rs[a]), {
      type: "op",
      mode: e.mode,
      limits: !1,
      parentIsSupSub: !1,
      symbol: !0,
      name: a
    };
  }
});
var Da = (r, e) => {
  var t, a, n = !1, s;
  r.type === "supsub" ? (t = r.sup, a = r.sub, s = j(r.base, "operatorname"), n = !0) : s = j(r, "operatorname");
  var u;
  if (s.body.length > 0) {
    for (var o = s.body.map((b) => {
      var k = "text" in b ? b.text : void 0;
      return typeof k == "string" ? {
        type: "textord",
        mode: b.mode,
        text: k
      } : b;
    }), h = o0(o, e.withFont("mathrm"), !0), m = 0; m < h.length; m++) {
      var v = h[m];
      v instanceof z0 && (v.text = v.text.replace(/\u2212/, "-").replace(/\u2217/, "*"));
    }
    u = F(["mop"], h, e);
  } else
    u = F(["mop"], [], e);
  return n ? Na(u, t, a, e, e.style, 0, 0) : u;
}, as = (r, e) => {
  for (var t = F0(r.body, e.withFont("mathrm")), a = !0, n = 0; n < t.length; n++) {
    var s = t[n];
    if (!(s instanceof oa)) if (s instanceof D)
      switch (s.type) {
        case "mi":
        case "mn":
        case "mspace":
        case "mtext":
          break;
        case "mo": {
          var u = s.children[0];
          s.children.length === 1 && u instanceof s0 ? u.text = u.text.replace(/\u2212/, "-").replace(/\u2217/, "*") : a = !1;
          break;
        }
        default:
          a = !1;
      }
    else
      a = !1;
  }
  if (a) {
    var o = t.map((v) => v.toText()).join("");
    t = [new s0(o)];
  }
  var h = new D("mi", t);
  h.setAttribute("mathvariant", "normal");
  var m = new D("mo", [C0("⁡", "text")]);
  return r.parentIsSupSub ? new D("mrow", [h, m]) : ua([h, m]);
};
O({
  type: "operatorname",
  names: ["\\operatorname@", "\\operatornamewithlimits"],
  numArgs: 1,
  handler: (r, e) => {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0];
    return {
      type: "operatorname",
      mode: t.mode,
      body: i0(n),
      alwaysHandleSupSub: a === "\\operatornamewithlimits",
      limits: !1,
      parentIsSupSub: !1
    };
  },
  htmlBuilder: Da,
  mathmlBuilder: as
});
d("\\operatorname", "\\@ifstar\\operatornamewithlimits\\operatorname@");
ue({
  type: "ordgroup",
  htmlBuilder(r, e) {
    return r.semisimple ? X0(o0(r.body, e, !1)) : F(["mord"], o0(r.body, e, !0), e);
  },
  mathmlBuilder(r, e) {
    return re(r.body, e, !0);
  }
});
O({
  type: "overline",
  names: ["\\overline"],
  numArgs: 1,
  handler(r, e) {
    var {
      parser: t
    } = r, a = e[0];
    return {
      type: "overline",
      mode: t.mode,
      body: a
    };
  },
  htmlBuilder(r, e) {
    var t = Y(r.body, e.havingCrampedStyle()), a = pe("overline-line", e), n = e.fontMetrics().defaultRuleThickness, s = V({
      positionType: "firstBaseline",
      children: [{
        type: "elem",
        elem: t
      }, {
        type: "kern",
        size: 3 * n
      }, {
        type: "elem",
        elem: a
      }, {
        type: "kern",
        size: n
      }]
    });
    return F(["mord", "overline"], [s], e);
  },
  mathmlBuilder(r, e) {
    var t = new D("mo", [new s0("‾")]);
    t.setAttribute("stretchy", "true");
    var a = new D("mover", [K(r.body, e), t]);
    return a.setAttribute("accent", "true"), a;
  }
});
O({
  type: "phantom",
  names: ["\\phantom"],
  numArgs: 1,
  allowedInText: !0,
  handler: (r, e) => {
    var {
      parser: t
    } = r, a = e[0];
    return {
      type: "phantom",
      mode: t.mode,
      body: i0(a)
    };
  },
  htmlBuilder: (r, e) => {
    var t = o0(r.body, e.withPhantom(), !1);
    return X0(t);
  },
  mathmlBuilder: (r, e) => {
    var t = F0(r.body, e);
    return new D("mphantom", t);
  }
});
d("\\hphantom", "\\smash{\\phantom{#1}}");
O({
  type: "vphantom",
  names: ["\\vphantom"],
  numArgs: 1,
  allowedInText: !0,
  handler: (r, e) => {
    var {
      parser: t
    } = r, a = e[0];
    return {
      type: "vphantom",
      mode: t.mode,
      body: a
    };
  },
  htmlBuilder: (r, e) => {
    var t = F(["inner"], [Y(r.body, e.withPhantom())]), a = F(["fix"], []);
    return F(["mord", "rlap"], [t, a], e);
  },
  mathmlBuilder: (r, e) => {
    var t = F0(i0(r.body), e), a = new D("mphantom", t), n = new D("mpadded", [a]);
    return n.setAttribute("width", "0px"), n;
  }
});
O({
  type: "raisebox",
  names: ["\\raisebox"],
  numArgs: 2,
  argTypes: ["size", "hbox"],
  allowedInText: !0,
  handler(r, e) {
    var {
      parser: t
    } = r, a = j(e[0], "size").value, n = e[1];
    return {
      type: "raisebox",
      mode: t.mode,
      dy: a,
      body: n
    };
  },
  htmlBuilder(r, e) {
    var t = Y(r.body, e), a = a0(r.dy, e);
    return V({
      positionType: "shift",
      positionData: -a,
      children: [{
        type: "elem",
        elem: t
      }]
    });
  },
  mathmlBuilder(r, e) {
    var t = new D("mpadded", [K(r.body, e)]), a = r.dy.number + r.dy.unit;
    return t.setAttribute("voffset", a), t;
  }
});
O({
  type: "internal",
  names: ["\\relax"],
  numArgs: 0,
  allowedInText: !0,
  allowedInArgument: !0,
  handler(r) {
    var {
      parser: e
    } = r;
    return {
      type: "internal",
      mode: e.mode
    };
  }
});
O({
  type: "rule",
  names: ["\\rule"],
  numArgs: 2,
  numOptionalArgs: 1,
  allowedInText: !0,
  allowedInMath: !0,
  argTypes: ["size", "size", "size"],
  handler(r, e, t) {
    var {
      parser: a
    } = r, n = t[0], s = j(e[0], "size"), u = j(e[1], "size");
    return {
      type: "rule",
      mode: a.mode,
      shift: n && j(n, "size").value,
      width: s.value,
      height: u.value
    };
  },
  htmlBuilder(r, e) {
    var t = F(["mord", "rule"], [], e), a = a0(r.width, e), n = a0(r.height, e), s = r.shift ? a0(r.shift, e) : 0;
    return t.style.borderRightWidth = B(a), t.style.borderTopWidth = B(n), t.style.bottom = B(s), t.width = a, t.height = n + s, t.depth = -s, t.maxFontSize = n * 1.125 * e.sizeMultiplier, t;
  },
  mathmlBuilder(r, e) {
    var t = a0(r.width, e), a = a0(r.height, e), n = r.shift ? a0(r.shift, e) : 0, s = e.color && e.getColor() || "black", u = new D("mspace");
    u.setAttribute("mathbackground", s), u.setAttribute("width", B(t)), u.setAttribute("height", B(a));
    var o = new D("mpadded", [u]);
    return n >= 0 ? o.setAttribute("height", B(n)) : (o.setAttribute("height", B(n)), o.setAttribute("depth", B(-n))), o.setAttribute("voffset", B(n)), o;
  }
});
function Ba(r, e, t) {
  for (var a = o0(r, e, !1), n = e.sizeMultiplier / t.sizeMultiplier, s = 0; s < a.length; s++) {
    var u = a[s].classes.indexOf("sizing");
    u < 0 ? Array.prototype.push.apply(a[s].classes, e.sizingClasses(t)) : a[s].classes[u + 1] === "reset-size" + e.size && (a[s].classes[u + 1] = "reset-size" + t.size), a[s].height *= n, a[s].depth *= n;
  }
  return X0(a);
}
var qr = ["\\tiny", "\\sixptsize", "\\scriptsize", "\\footnotesize", "\\small", "\\normalsize", "\\large", "\\Large", "\\LARGE", "\\huge", "\\Huge"], ns = (r, e) => {
  var t = e.havingSize(r.size);
  return Ba(r.body, t, e);
};
O({
  type: "sizing",
  names: qr,
  numArgs: 0,
  allowedInText: !0,
  handler: (r, e) => {
    var {
      breakOnTokenText: t,
      funcName: a,
      parser: n
    } = r, s = n.parseExpression(!1, t);
    return {
      type: "sizing",
      mode: n.mode,
      // Figure out what size to use based on the list of functions above
      size: qr.indexOf(a) + 1,
      body: s
    };
  },
  htmlBuilder: ns,
  mathmlBuilder: (r, e) => {
    var t = e.havingSize(r.size), a = F0(r.body, t), n = new D("mstyle", a);
    return n.setAttribute("mathsize", B(t.sizeMultiplier)), n;
  }
});
O({
  type: "smash",
  names: ["\\smash"],
  numArgs: 1,
  numOptionalArgs: 1,
  allowedInText: !0,
  handler: (r, e, t) => {
    var {
      parser: a
    } = r, n = !1, s = !1, u = t[0] && j(t[0], "ordgroup");
    if (u)
      for (var o, h = 0; h < u.body.length; ++h) {
        var m = u.body[h];
        if (o = Ke(m).text, o === "t")
          n = !0;
        else if (o === "b")
          s = !0;
        else {
          n = !1, s = !1;
          break;
        }
      }
    else
      n = !0, s = !0;
    var v = e[0];
    return {
      type: "smash",
      mode: a.mode,
      body: v,
      smashHeight: n,
      smashDepth: s
    };
  },
  htmlBuilder: (r, e) => {
    var t = F([], [Y(r.body, e)]);
    if (!r.smashHeight && !r.smashDepth)
      return t;
    if (r.smashHeight && (t.height = 0), r.smashDepth && (t.depth = 0), r.smashHeight && r.smashDepth)
      return F(["mord", "smash"], [t], e);
    if (t.children)
      for (var a = 0; a < t.children.length; a++)
        r.smashHeight && (t.children[a].height = 0), r.smashDepth && (t.children[a].depth = 0);
    var n = V({
      positionType: "firstBaseline",
      children: [{
        type: "elem",
        elem: t
      }]
    });
    return F(["mord"], [n], e);
  },
  mathmlBuilder: (r, e) => {
    var t = new D("mpadded", [K(r.body, e)]);
    return r.smashHeight && t.setAttribute("height", "0px"), r.smashDepth && t.setAttribute("depth", "0px"), t;
  }
});
O({
  type: "sqrt",
  names: ["\\sqrt"],
  numArgs: 1,
  numOptionalArgs: 1,
  handler(r, e, t) {
    var {
      parser: a
    } = r, n = t[0], s = e[0];
    return {
      type: "sqrt",
      mode: a.mode,
      body: s,
      index: n
    };
  },
  htmlBuilder(r, e) {
    var t = Y(r.body, e.havingCrampedStyle());
    t.height === 0 && (t.height = e.fontMetrics().xHeight), t = ve(t, e);
    var a = e.fontMetrics(), n = a.defaultRuleThickness, s = n;
    e.style.id < U.TEXT.id && (s = e.fontMetrics().xHeight);
    var u = n + s / 4, o = t.height + t.depth + u + n, {
      span: h,
      ruleWidth: m,
      advanceWidth: v
    } = Li(o, e), b = h.height - m;
    b > t.height + t.depth + u && (u = (u + b - t.height - t.depth) / 2);
    var k = h.height - t.height - u - m;
    t.style.paddingLeft = B(v);
    var S = V({
      positionType: "firstBaseline",
      children: [{
        type: "elem",
        elem: t,
        wrapperClasses: ["svg-align"]
      }, {
        type: "kern",
        size: -(t.height + k)
      }, {
        type: "elem",
        elem: h
      }, {
        type: "kern",
        size: m
      }]
    });
    if (r.index) {
      var _ = e.havingStyle(U.SCRIPTSCRIPT), C = Y(r.index, _, e), N = 0.6 * (S.height - S.depth), I = V({
        positionType: "shift",
        positionData: -N,
        children: [{
          type: "elem",
          elem: C
        }]
      }), x = F(["root"], [I]);
      return F(["mord", "sqrt"], [x, S], e);
    } else
      return F(["mord", "sqrt"], [S], e);
  },
  mathmlBuilder(r, e) {
    var {
      body: t,
      index: a
    } = r;
    return a ? new D("mroot", [K(t, e), K(a, e)]) : new D("msqrt", [K(t, e)]);
  }
});
var Ht = {
  display: U.DISPLAY,
  text: U.TEXT,
  script: U.SCRIPT,
  scriptscript: U.SCRIPTSCRIPT
};
function is(r) {
  return r in Ht;
}
O({
  type: "styling",
  names: ["\\displaystyle", "\\textstyle", "\\scriptstyle", "\\scriptscriptstyle"],
  numArgs: 0,
  allowedInText: !0,
  primitive: !0,
  handler(r, e) {
    var {
      breakOnTokenText: t,
      funcName: a,
      parser: n
    } = r, s = n.parseExpression(!0, t), u = a.slice(1, a.length - 5);
    if (!is(u))
      throw new Error("Unknown style: " + u);
    return {
      type: "styling",
      mode: n.mode,
      // Figure out what style to use by pulling out the style from
      // the function name
      style: u,
      body: s
    };
  },
  htmlBuilder(r, e) {
    var t = Ht[r.style], a = e.havingStyle(t);
    return r.resetFont && (a = a.withFont("")), Ba(r.body, a, e);
  },
  mathmlBuilder(r, e) {
    var t = Ht[r.style], a = e.havingStyle(t);
    r.resetFont && (a = a.withFont(""));
    var n = F0(r.body, a), s = new D("mstyle", n), u = {
      display: ["0", "true"],
      text: ["0", "false"],
      script: ["1", "false"],
      scriptscript: ["2", "false"]
    }, o = u[r.style];
    return s.setAttribute("scriptlevel", o[0]), s.setAttribute("displaystyle", o[1]), s;
  }
});
var ss = function(e, t) {
  var a = e.base;
  if (a)
    if (a.type === "op") {
      var n = a.limits && (t.style.size === U.DISPLAY.size || a.alwaysHandleSupSub);
      return n ? Ea : null;
    } else if (a.type === "operatorname") {
      var s = a.alwaysHandleSupSub && (t.style.size === U.DISPLAY.size || a.limits);
      return s ? Da : null;
    } else {
      if (a.type === "accent")
        return U0(a.base) ? ma : null;
      if (a.type === "horizBrace") {
        var u = !e.sub;
        return u === a.isOver ? Fa : null;
      } else
        return null;
    }
  else return null;
};
ue({
  type: "supsub",
  htmlBuilder(r, e) {
    var t = ss(r, e);
    if (t)
      return t(r, e);
    var {
      base: a,
      sup: n,
      sub: s
    } = r, u = Y(a, e), o, h, m = e.fontMetrics(), v = 0, b = 0, k = a && U0(a);
    if (n) {
      var S = e.havingStyle(e.style.sup());
      o = Y(n, S, e), k || (v = u.height - S.fontMetrics().supDrop * S.sizeMultiplier / e.sizeMultiplier);
    }
    if (s) {
      var _ = e.havingStyle(e.style.sub());
      h = Y(s, _, e), k || (b = u.depth + _.fontMetrics().subDrop * _.sizeMultiplier / e.sizeMultiplier);
    }
    var C;
    e.style === U.DISPLAY ? C = m.sup1 : e.style.cramped ? C = m.sup3 : C = m.sup2;
    var N = e.sizeMultiplier, I = B(0.5 / m.ptPerEm / N), x = null;
    if (h) {
      var w = r.base && r.base.type === "op" && r.base.name && (r.base.name === "\\oiint" || r.base.name === "\\oiiint");
      if (u instanceof z0 || w) {
        var A;
        x = B(-((A = u.italic) != null ? A : 0));
      }
    }
    var T;
    if (o && h) {
      v = Math.max(v, C, o.depth + 0.25 * m.xHeight), b = Math.max(b, m.sub2);
      var z = m.defaultRuleThickness, q = 4 * z;
      if (v - o.depth - (h.height - b) < q) {
        b = q - (v - o.depth) + h.height;
        var H = 0.8 * m.xHeight - (v - o.depth);
        H > 0 && (v += H, b -= H);
      }
      var R = [{
        type: "elem",
        elem: h,
        shift: b,
        marginRight: I,
        marginLeft: x
      }, {
        type: "elem",
        elem: o,
        shift: -v,
        marginRight: I
      }];
      T = V({
        positionType: "individualShift",
        children: R
      });
    } else if (h) {
      b = Math.max(b, m.sub1, h.height - 0.8 * m.xHeight);
      var X = [{
        type: "elem",
        elem: h,
        marginLeft: x,
        marginRight: I
      }];
      T = V({
        positionType: "shift",
        positionData: b,
        children: X
      });
    } else if (o)
      v = Math.max(v, C, o.depth + 0.25 * m.xHeight), T = V({
        positionType: "shift",
        positionData: -v,
        children: [{
          type: "elem",
          elem: o,
          marginRight: I
        }]
      });
    else
      throw new Error("supsub must have either sup or sub.");
    var W = Rt(u, "right") || "mord";
    return F([W], [u, F(["msupsub"], [T])], e);
  },
  mathmlBuilder(r, e) {
    var t = !1, a, n;
    r.base && r.base.type === "horizBrace" && (n = !!r.sup, n === r.base.isOver && (t = !0, a = r.base.isOver)), r.base && (r.base.type === "op" || r.base.type === "operatorname") && (r.base.parentIsSupSub = !0);
    var s = [K(r.base, e)];
    r.sub && s.push(K(r.sub, e)), r.sup && s.push(K(r.sup, e));
    var u;
    if (t)
      u = a ? "mover" : "munder";
    else if (r.sub)
      if (r.sup) {
        var m = r.base;
        m && m.type === "op" && m.limits && e.style === U.DISPLAY || m && m.type === "operatorname" && m.alwaysHandleSupSub && (e.style === U.DISPLAY || m.limits) ? u = "munderover" : u = "msubsup";
      } else {
        var h = r.base;
        h && h.type === "op" && h.limits && (e.style === U.DISPLAY || h.alwaysHandleSupSub) || h && h.type === "operatorname" && h.alwaysHandleSupSub && (h.limits || e.style === U.DISPLAY) ? u = "munder" : u = "msub";
      }
    else {
      var o = r.base;
      o && o.type === "op" && o.limits && (e.style === U.DISPLAY || o.alwaysHandleSupSub) || o && o.type === "operatorname" && o.alwaysHandleSupSub && (o.limits || e.style === U.DISPLAY) ? u = "mover" : u = "msup";
    }
    return new D(u, s);
  }
});
ue({
  type: "atom",
  htmlBuilder(r, e) {
    return Vt(r.text, r.mode, e, ["m" + r.family]);
  },
  mathmlBuilder(r, e) {
    var t = new D("mo", [C0(r.text, r.mode)]);
    if (r.family === "bin") {
      var a = Zt(r, e);
      a === "bold-italic" && t.setAttribute("mathvariant", a);
    } else r.family === "punct" ? t.setAttribute("separator", "true") : (r.family === "open" || r.family === "close") && t.setAttribute("stretchy", "false");
    return t;
  }
});
var Ia = {
  mi: "italic",
  mn: "normal",
  mtext: "normal"
};
ue({
  type: "mathord",
  htmlBuilder(r, e) {
    return Ye(r, e);
  },
  mathmlBuilder(r, e) {
    var t = new D("mi", [C0(r.text, r.mode, e)]), a = Zt(r, e) || "italic";
    return a !== Ia[t.type] && t.setAttribute("mathvariant", a), t;
  }
});
ue({
  type: "textord",
  htmlBuilder(r, e) {
    return Ye(r, e);
  },
  mathmlBuilder(r, e) {
    var t = C0(r.text, r.mode, e), a = Zt(r, e) || "normal", n;
    return r.mode === "text" ? n = new D("mtext", [t]) : /[0-9]/.test(r.text) ? n = new D("mn", [t]) : r.text === "\\prime" ? n = new D("mo", [t]) : n = new D("mi", [t]), a !== Ia[n.type] && n.setAttribute("mathvariant", a), n;
  }
});
var bt = {
  "\\nobreak": "nobreak",
  "\\allowbreak": "allowbreak"
}, yt = {
  " ": {},
  "\\ ": {},
  "~": {
    className: "nobreak"
  },
  "\\space": {},
  "\\nobreakspace": {
    className: "nobreak"
  }
};
ue({
  type: "spacing",
  htmlBuilder(r, e) {
    if (yt.hasOwnProperty(r.text)) {
      var t = yt[r.text].className || "";
      if (r.mode === "text") {
        var a = Ye(r, e);
        return a.classes.push(t), a;
      } else
        return F(["mspace", t], [Vt(r.text, r.mode, e)], e);
    } else {
      if (bt.hasOwnProperty(r.text))
        return F(["mspace", bt[r.text]], [], e);
      throw new E('Unknown type of space "' + r.text + '"');
    }
  },
  mathmlBuilder(r, e) {
    var t;
    if (yt.hasOwnProperty(r.text))
      t = new D("mtext", [new s0(" ")]);
    else {
      if (bt.hasOwnProperty(r.text))
        return new D("mspace");
      throw new E('Unknown type of space "' + r.text + '"');
    }
    return t;
  }
});
var _r = () => {
  var r = new D("mtd", []);
  return r.setAttribute("width", "50%"), r;
};
ue({
  type: "tag",
  mathmlBuilder(r, e) {
    var t = new D("mtable", [new D("mtr", [_r(), new D("mtd", [re(r.body, e)]), _r(), new D("mtd", [re(r.tag, e)])])]);
    return t.setAttribute("width", "100%"), t;
  }
});
var Or = {
  "\\text": void 0,
  "\\textrm": "textrm",
  "\\textsf": "textsf",
  "\\texttt": "texttt",
  "\\textnormal": "textrm"
}, Lr = {
  "\\textbf": "textbf",
  "\\textmd": "textmd"
}, ls = {
  "\\textit": "textit",
  "\\textup": "textup"
}, Hr = (r, e) => {
  var t = r.font;
  if (t) {
    if (Or[t])
      return e.withTextFontFamily(Or[t]);
    if (Lr[t])
      return e.withTextFontWeight(Lr[t]);
    if (t === "\\emph")
      return e.fontShape === "textit" ? e.withTextFontShape("textup") : e.withTextFontShape("textit");
  } else return e;
  return e.withTextFontShape(ls[t]);
};
O({
  type: "text",
  names: [
    // Font families
    "\\text",
    "\\textrm",
    "\\textsf",
    "\\texttt",
    "\\textnormal",
    // Font weights
    "\\textbf",
    "\\textmd",
    // Font Shapes
    "\\textit",
    "\\textup",
    "\\emph"
  ],
  numArgs: 1,
  argTypes: ["text"],
  allowedInArgument: !0,
  allowedInText: !0,
  handler(r, e) {
    var {
      parser: t,
      funcName: a
    } = r, n = e[0];
    return {
      type: "text",
      mode: t.mode,
      body: i0(n),
      font: a
    };
  },
  htmlBuilder(r, e) {
    var t = Hr(r, e), a = o0(r.body, t, !0);
    return F(["mord", "text"], a, t);
  },
  mathmlBuilder(r, e) {
    var t = Hr(r, e);
    return re(r.body, t);
  }
});
O({
  type: "underline",
  names: ["\\underline"],
  numArgs: 1,
  allowedInText: !0,
  handler(r, e) {
    var {
      parser: t
    } = r;
    return {
      type: "underline",
      mode: t.mode,
      body: e[0]
    };
  },
  htmlBuilder(r, e) {
    var t = Y(r.body, e), a = pe("underline-line", e), n = e.fontMetrics().defaultRuleThickness, s = V({
      positionType: "top",
      positionData: t.height,
      children: [{
        type: "kern",
        size: n
      }, {
        type: "elem",
        elem: a
      }, {
        type: "kern",
        size: 3 * n
      }, {
        type: "elem",
        elem: t
      }]
    });
    return F(["mord", "underline"], [s], e);
  },
  mathmlBuilder(r, e) {
    var t = new D("mo", [new s0("‾")]);
    t.setAttribute("stretchy", "true");
    var a = new D("munder", [K(r.body, e), t]);
    return a.setAttribute("accentunder", "true"), a;
  }
});
O({
  type: "vcenter",
  names: ["\\vcenter"],
  numArgs: 1,
  argTypes: ["original"],
  // In LaTeX, \vcenter can act only on a box.
  allowedInText: !1,
  handler(r, e) {
    var {
      parser: t
    } = r;
    return {
      type: "vcenter",
      mode: t.mode,
      body: e[0]
    };
  },
  htmlBuilder(r, e) {
    var t = Y(r.body, e), a = e.fontMetrics().axisHeight, n = 0.5 * (t.height - a - (t.depth + a));
    return V({
      positionType: "shift",
      positionData: n,
      children: [{
        type: "elem",
        elem: t
      }]
    });
  },
  mathmlBuilder(r, e) {
    var t = new D("mpadded", [K(r.body, e)], ["vcenter"]);
    return new D("mrow", [t]);
  }
});
O({
  type: "verb",
  names: ["\\verb"],
  numArgs: 0,
  allowedInText: !0,
  handler(r, e, t) {
    throw new E("\\verb ended by end of line instead of matching delimiter");
  },
  htmlBuilder(r, e) {
    for (var t = $r(r), a = [], n = e.havingStyle(e.style.text()), s = 0; s < t.length; s++) {
      var u = t[s];
      u === "~" && (u = "\\textasciitilde"), a.push(p0(u, "Typewriter-Regular", r.mode, n, ["mord", "texttt"]));
    }
    return F(["mord", "text"].concat(n.sizingClasses(e)), ra(a), n);
  },
  mathmlBuilder(r, e) {
    var t = new s0($r(r)), a = new D("mtext", [t]);
    return a.setAttribute("mathvariant", "monospace"), a;
  }
});
var $r = (r) => r.body.replace(/ /g, r.star ? "␣" : " "), J0 = sa, Ra = `[ \r
	]`, us = "\\\\[a-zA-Z@]+", os = "\\\\[^\uD800-\uDFFF]", cs = "(" + us + ")" + Ra + "*", hs = `\\\\(
|[ \r	]+
?)[ \r	]*`, $t = "[̀-ͯ]", ds = new RegExp($t + "+$"), ms = "(" + Ra + "+)|" + // whitespace
(hs + "|") + // \whitespace
"([!-\\[\\]-‧‪-퟿豈-￿]" + // single codepoint
($t + "*") + // ...plus accents
"|[\uD800-\uDBFF][\uDC00-\uDFFF]" + // surrogate pair
($t + "*") + // ...plus accents
"|\\\\verb\\*([^]).*?\\4|\\\\verb([^*a-zA-Z]).*?\\5" + // \verb unstarred
("|" + cs) + // \macroName + spaces
("|" + os + ")");
class Pr {
  constructor(e, t) {
    this.input = void 0, this.settings = void 0, this.tokenRegex = void 0, this.catcodes = void 0, this.input = e, this.settings = t, this.tokenRegex = new RegExp(ms, "g"), this.catcodes = {
      "%": 14,
      // comment character
      "~": 13
      // active character
    };
  }
  setCatcode(e, t) {
    this.catcodes[e] = t;
  }
  /**
   * This function lexes a single token.
   */
  lex() {
    var e = this.input, t = this.tokenRegex.lastIndex;
    if (t === e.length)
      return new T0("EOF", new y0(this, t, t));
    var a = this.tokenRegex.exec(e);
    if (a === null || a.index !== t)
      throw new E("Unexpected character: '" + e[t] + "'", new T0(e[t], new y0(this, t, t + 1)));
    var n = a[6] || a[3] || (a[2] ? "\\ " : " ");
    if (this.catcodes[n] === 14) {
      var s = e.indexOf(`
`, this.tokenRegex.lastIndex);
      return s === -1 ? (this.tokenRegex.lastIndex = e.length, this.settings.reportNonstrict("commentAtEnd", "% comment has no terminating newline; LaTeX would fail because of commenting the end of math mode (e.g. $)")) : this.tokenRegex.lastIndex = s + 1, this.lex();
    }
    return new T0(n, new y0(this, t, this.tokenRegex.lastIndex));
  }
}
class fs {
  /**
   * Both arguments are optional.  The first argument is an object of
   * built-in mappings which never change.  The second argument is an object
   * of initial (global-level) mappings, which will constantly change
   * according to any global/top-level `set`s done.
   */
  constructor(e, t) {
    e === void 0 && (e = {}), t === void 0 && (t = {}), this.current = void 0, this.builtins = void 0, this.undefStack = void 0, this.current = t, this.builtins = e, this.undefStack = [];
  }
  /**
   * Start a new nested group, affecting future local `set`s.
   */
  beginGroup() {
    this.undefStack.push({});
  }
  /**
   * End current nested group, restoring values before the group began.
   */
  endGroup() {
    if (this.undefStack.length === 0)
      throw new E("Unbalanced namespace destruction: attempt to pop global namespace; please report this as a bug");
    var e = this.undefStack.pop();
    for (var t in e)
      e.hasOwnProperty(t) && (e[t] == null ? delete this.current[t] : this.current[t] = e[t]);
  }
  /**
   * Ends all currently nested groups (if any), restoring values before the
   * groups began.  Useful in case of an error in the middle of parsing.
   */
  endGroups() {
    for (; this.undefStack.length > 0; )
      this.endGroup();
  }
  /**
   * Detect whether `name` has a definition.  Equivalent to
   * `get(name) != null`.
   */
  has(e) {
    return this.current.hasOwnProperty(e) || this.builtins.hasOwnProperty(e);
  }
  /**
   * Get the current value of a name, or `undefined` if there is no value.
   *
   * Note: Do not use `if (namespace.get(...))` to detect whether a macro
   * is defined, as the definition may be the empty string which evaluates
   * to `false` in JavaScript.  Use `if (namespace.get(...) != null)` or
   * `if (namespace.has(...))`.
   */
  get(e) {
    return this.current.hasOwnProperty(e) ? this.current[e] : this.builtins[e];
  }
  /**
   * Set the current value of a name, and optionally set it globally too.
   * Local set() sets the current value and (when appropriate) adds an undo
   * operation to the undo stack.  Global set() may change the undo
   * operation at every level, so takes time linear in their number.
   * A value of undefined means to delete existing definitions.
   */
  set(e, t, a) {
    if (a === void 0 && (a = !1), a) {
      for (var n = 0; n < this.undefStack.length; n++)
        delete this.undefStack[n][e];
      this.undefStack.length > 0 && (this.undefStack[this.undefStack.length - 1][e] = t);
    } else {
      var s = this.undefStack[this.undefStack.length - 1];
      s && !s.hasOwnProperty(e) && (s[e] = this.current[e]);
    }
    t == null ? delete this.current[e] : this.current[e] = t;
  }
}
var ps = Ta;
d("\\noexpand", function(r) {
  var e = r.popToken();
  return r.isExpandable(e.text) && (e.noexpand = !0, e.treatAsRelax = !0), {
    tokens: [e],
    numArgs: 0
  };
});
d("\\expandafter", function(r) {
  var e = r.popToken();
  return r.expandOnce(!0), {
    tokens: [e],
    numArgs: 0
  };
});
d("\\@firstoftwo", function(r) {
  var e = r.consumeArgs(2);
  return {
    tokens: e[0],
    numArgs: 0
  };
});
d("\\@secondoftwo", function(r) {
  var e = r.consumeArgs(2);
  return {
    tokens: e[1],
    numArgs: 0
  };
});
d("\\@ifnextchar", function(r) {
  var e = r.consumeArgs(3);
  r.consumeSpaces();
  var t = r.future();
  return e[0].length === 1 && e[0][0].text === t.text ? {
    tokens: e[1],
    numArgs: 0
  } : {
    tokens: e[2],
    numArgs: 0
  };
});
d("\\@ifstar", "\\@ifnextchar *{\\@firstoftwo{#1}}");
d("\\TextOrMath", function(r) {
  var e = r.consumeArgs(2);
  return r.mode === "text" ? {
    tokens: e[0],
    numArgs: 0
  } : {
    tokens: e[1],
    numArgs: 0
  };
});
var Gr = {
  0: 0,
  1: 1,
  2: 2,
  3: 3,
  4: 4,
  5: 5,
  6: 6,
  7: 7,
  8: 8,
  9: 9,
  a: 10,
  A: 10,
  b: 11,
  B: 11,
  c: 12,
  C: 12,
  d: 13,
  D: 13,
  e: 14,
  E: 14,
  f: 15,
  F: 15
};
d("\\char", function(r) {
  var e = r.popToken(), t, a = 0;
  if (e.text === "'")
    t = 8, e = r.popToken();
  else if (e.text === '"')
    t = 16, e = r.popToken();
  else if (e.text === "`")
    if (e = r.popToken(), e.text[0] === "\\")
      a = e.text.charCodeAt(1);
    else {
      if (e.text === "EOF")
        throw new E("\\char` missing argument");
      a = e.text.charCodeAt(0);
    }
  else
    t = 10;
  if (t) {
    if (a = Gr[e.text], a == null || a >= t)
      throw new E("Invalid base-" + t + " digit " + e.text);
    for (var n; (n = Gr[r.future().text]) != null && n < t; )
      a *= t, a += n, r.popToken();
  }
  return "\\@char{" + a + "}";
});
var er = (r, e, t, a) => {
  var n = r.consumeArg().tokens;
  if (n.length !== 1)
    throw new E("\\newcommand's first argument must be a macro name");
  var s = n[0].text, u = r.isDefined(s);
  if (u && !e)
    throw new E("\\newcommand{" + s + "} attempting to redefine " + (s + "; use \\renewcommand"));
  if (!u && !t)
    throw new E("\\renewcommand{" + s + "} when command " + s + " does not yet exist; use \\newcommand");
  var o = 0;
  if (n = r.consumeArg().tokens, n.length === 1 && n[0].text === "[") {
    for (var h = "", m = r.expandNextToken(); m.text !== "]" && m.text !== "EOF"; )
      h += m.text, m = r.expandNextToken();
    if (!h.match(/^\s*[0-9]+\s*$/))
      throw new E("Invalid number of arguments: " + h);
    o = parseInt(h), n = r.consumeArg().tokens;
  }
  return u && a || r.macros.set(s, {
    tokens: n,
    numArgs: o
  }), "";
};
d("\\newcommand", (r) => er(r, !1, !0, !1));
d("\\renewcommand", (r) => er(r, !0, !1, !1));
d("\\providecommand", (r) => er(r, !0, !0, !0));
d("\\message", (r) => {
  var e = r.consumeArgs(1)[0];
  return console.log(e.reverse().map((t) => t.text).join("")), "";
});
d("\\errmessage", (r) => {
  var e = r.consumeArgs(1)[0];
  return console.error(e.reverse().map((t) => t.text).join("")), "";
});
d("\\show", (r) => {
  var e = r.popToken(), t = e.text;
  return console.log(e, r.macros.get(t), J0[t], Q.math[t], Q.text[t]), "";
});
d("\\bgroup", "{");
d("\\egroup", "}");
d("~", "\\nobreakspace");
d("\\lq", "`");
d("\\rq", "'");
d("\\aa", "\\r a");
d("\\AA", "\\r A");
d("\\textcopyright", "\\html@mathml{\\textcircled{c}}{\\char`©}");
d("\\copyright", "\\TextOrMath{\\textcopyright}{\\text{\\textcopyright}}");
d("\\textregistered", "\\html@mathml{\\textcircled{\\scriptsize R}}{\\char`®}");
d("ℬ", "\\mathscr{B}");
d("ℰ", "\\mathscr{E}");
d("ℱ", "\\mathscr{F}");
d("ℋ", "\\mathscr{H}");
d("ℐ", "\\mathscr{I}");
d("ℒ", "\\mathscr{L}");
d("ℳ", "\\mathscr{M}");
d("ℛ", "\\mathscr{R}");
d("ℭ", "\\mathfrak{C}");
d("ℌ", "\\mathfrak{H}");
d("ℨ", "\\mathfrak{Z}");
d("\\Bbbk", "\\Bbb{k}");
d("\\llap", "\\mathllap{\\textrm{#1}}");
d("\\rlap", "\\mathrlap{\\textrm{#1}}");
d("\\clap", "\\mathclap{\\textrm{#1}}");
d("\\mathstrut", "\\vphantom{(}");
d("\\underbar", "\\underline{\\text{#1}}");
d("\\not", '\\html@mathml{\\mathrel{\\mathrlap\\@not}\\nobreak}{\\char"338}');
d("\\neq", "\\html@mathml{\\mathrel{\\not=}}{\\mathrel{\\char`≠}}");
d("\\ne", "\\neq");
d("≠", "\\neq");
d("\\notin", "\\html@mathml{\\mathrel{{\\in}\\mathllap{/\\mskip1mu}}}{\\mathrel{\\char`∉}}");
d("∉", "\\notin");
d("≘", "\\html@mathml{\\mathrel{=\\kern{-1em}\\raisebox{0.4em}{$\\scriptsize\\frown$}}}{\\mathrel{\\char`≘}}");
d("≙", "\\html@mathml{\\stackrel{\\tiny\\wedge}{=}}{\\mathrel{\\char`≘}}");
d("≚", "\\html@mathml{\\stackrel{\\tiny\\vee}{=}}{\\mathrel{\\char`≚}}");
d("≛", "\\html@mathml{\\stackrel{\\scriptsize\\star}{=}}{\\mathrel{\\char`≛}}");
d("≝", "\\html@mathml{\\stackrel{\\tiny\\mathrm{def}}{=}}{\\mathrel{\\char`≝}}");
d("≞", "\\html@mathml{\\stackrel{\\tiny\\mathrm{m}}{=}}{\\mathrel{\\char`≞}}");
d("≟", "\\html@mathml{\\stackrel{\\tiny?}{=}}{\\mathrel{\\char`≟}}");
d("⟂", "\\perp");
d("‼", "\\mathclose{!\\mkern-0.8mu!}");
d("∌", "\\notni");
d("⌜", "\\ulcorner");
d("⌝", "\\urcorner");
d("⌞", "\\llcorner");
d("⌟", "\\lrcorner");
d("©", "\\copyright");
d("®", "\\textregistered");
d("\\ulcorner", '\\html@mathml{\\@ulcorner}{\\mathop{\\char"231c}}');
d("\\urcorner", '\\html@mathml{\\@urcorner}{\\mathop{\\char"231d}}');
d("\\llcorner", '\\html@mathml{\\@llcorner}{\\mathop{\\char"231e}}');
d("\\lrcorner", '\\html@mathml{\\@lrcorner}{\\mathop{\\char"231f}}');
d("\\vdots", "{\\varvdots\\rule{0pt}{15pt}}");
d("⋮", "\\vdots");
d("\\varGamma", "\\mathit{\\Gamma}");
d("\\varDelta", "\\mathit{\\Delta}");
d("\\varTheta", "\\mathit{\\Theta}");
d("\\varLambda", "\\mathit{\\Lambda}");
d("\\varXi", "\\mathit{\\Xi}");
d("\\varPi", "\\mathit{\\Pi}");
d("\\varSigma", "\\mathit{\\Sigma}");
d("\\varUpsilon", "\\mathit{\\Upsilon}");
d("\\varPhi", "\\mathit{\\Phi}");
d("\\varPsi", "\\mathit{\\Psi}");
d("\\varOmega", "\\mathit{\\Omega}");
d("\\substack", "\\begin{subarray}{c}#1\\end{subarray}");
d("\\colon", "\\nobreak\\mskip2mu\\mathpunct{}\\mathchoice{\\mkern-3mu}{\\mkern-3mu}{}{}{:}\\mskip6mu\\relax");
d("\\boxed", "\\fbox{$\\displaystyle{#1}$}");
d("\\iff", "\\DOTSB\\;\\Longleftrightarrow\\;");
d("\\implies", "\\DOTSB\\;\\Longrightarrow\\;");
d("\\impliedby", "\\DOTSB\\;\\Longleftarrow\\;");
d("\\dddot", "{\\overset{\\raisebox{-0.1ex}{\\normalsize ...}}{#1}}");
d("\\ddddot", "{\\overset{\\raisebox{-0.1ex}{\\normalsize ....}}{#1}}");
var Ur = {
  ",": "\\dotsc",
  "\\not": "\\dotsb",
  // \keybin@ checks for the following:
  "+": "\\dotsb",
  "=": "\\dotsb",
  "<": "\\dotsb",
  ">": "\\dotsb",
  "-": "\\dotsb",
  "*": "\\dotsb",
  ":": "\\dotsb",
  // Symbols whose definition starts with \DOTSB:
  "\\DOTSB": "\\dotsb",
  "\\coprod": "\\dotsb",
  "\\bigvee": "\\dotsb",
  "\\bigwedge": "\\dotsb",
  "\\biguplus": "\\dotsb",
  "\\bigcap": "\\dotsb",
  "\\bigcup": "\\dotsb",
  "\\prod": "\\dotsb",
  "\\sum": "\\dotsb",
  "\\bigotimes": "\\dotsb",
  "\\bigoplus": "\\dotsb",
  "\\bigodot": "\\dotsb",
  "\\bigsqcup": "\\dotsb",
  "\\And": "\\dotsb",
  "\\longrightarrow": "\\dotsb",
  "\\Longrightarrow": "\\dotsb",
  "\\longleftarrow": "\\dotsb",
  "\\Longleftarrow": "\\dotsb",
  "\\longleftrightarrow": "\\dotsb",
  "\\Longleftrightarrow": "\\dotsb",
  "\\mapsto": "\\dotsb",
  "\\longmapsto": "\\dotsb",
  "\\hookrightarrow": "\\dotsb",
  "\\doteq": "\\dotsb",
  // Symbols whose definition starts with \mathbin:
  "\\mathbin": "\\dotsb",
  // Symbols whose definition starts with \mathrel:
  "\\mathrel": "\\dotsb",
  "\\relbar": "\\dotsb",
  "\\Relbar": "\\dotsb",
  "\\xrightarrow": "\\dotsb",
  "\\xleftarrow": "\\dotsb",
  // Symbols whose definition starts with \DOTSI:
  "\\DOTSI": "\\dotsi",
  "\\int": "\\dotsi",
  "\\oint": "\\dotsi",
  "\\iint": "\\dotsi",
  "\\iiint": "\\dotsi",
  "\\iiiint": "\\dotsi",
  "\\idotsint": "\\dotsi",
  // Symbols whose definition starts with \DOTSX:
  "\\DOTSX": "\\dotsx"
}, vs = /* @__PURE__ */ new Set(["bin", "rel"]);
d("\\dots", function(r) {
  var e = "\\dotso", t = r.expandAfterFuture().text;
  return t in Ur ? e = Ur[t] : (t.slice(0, 4) === "\\not" || t in Q.math && vs.has(Q.math[t].group)) && (e = "\\dotsb"), e;
});
var tr = {
  // \rightdelim@ checks for the following:
  ")": !0,
  "]": !0,
  "\\rbrack": !0,
  "\\}": !0,
  "\\rbrace": !0,
  "\\rangle": !0,
  "\\rceil": !0,
  "\\rfloor": !0,
  "\\rgroup": !0,
  "\\rmoustache": !0,
  "\\right": !0,
  "\\bigr": !0,
  "\\biggr": !0,
  "\\Bigr": !0,
  "\\Biggr": !0,
  // \extra@ also tests for the following:
  $: !0,
  // \extrap@ checks for the following:
  ";": !0,
  ".": !0,
  ",": !0
};
d("\\dotso", function(r) {
  var e = r.future().text;
  return e in tr ? "\\ldots\\," : "\\ldots";
});
d("\\dotsc", function(r) {
  var e = r.future().text;
  return e in tr && e !== "," ? "\\ldots\\," : "\\ldots";
});
d("\\cdots", function(r) {
  var e = r.future().text;
  return e in tr ? "\\@cdots\\," : "\\@cdots";
});
d("\\dotsb", "\\cdots");
d("\\dotsm", "\\cdots");
d("\\dotsi", "\\!\\cdots");
d("\\dotsx", "\\ldots\\,");
d("\\DOTSI", "\\relax");
d("\\DOTSB", "\\relax");
d("\\DOTSX", "\\relax");
d("\\tmspace", "\\TextOrMath{\\kern#1#3}{\\mskip#1#2}\\relax");
d("\\,", "\\tmspace+{3mu}{.1667em}");
d("\\thinspace", "\\,");
d("\\>", "\\mskip{4mu}");
d("\\:", "\\tmspace+{4mu}{.2222em}");
d("\\medspace", "\\:");
d("\\;", "\\tmspace+{5mu}{.2777em}");
d("\\thickspace", "\\;");
d("\\!", "\\tmspace-{3mu}{.1667em}");
d("\\negthinspace", "\\!");
d("\\negmedspace", "\\tmspace-{4mu}{.2222em}");
d("\\negthickspace", "\\tmspace-{5mu}{.277em}");
d("\\enspace", "\\kern.5em ");
d("\\enskip", "\\hskip.5em\\relax");
d("\\quad", "\\hskip1em\\relax");
d("\\qquad", "\\hskip2em\\relax");
d("\\tag", "\\@ifstar\\tag@literal\\tag@paren");
d("\\tag@paren", "\\tag@literal{({#1})}");
d("\\tag@literal", (r) => {
  if (r.macros.get("\\df@tag"))
    throw new E("Multiple \\tag");
  return "\\gdef\\df@tag{\\text{#1}}";
});
d("\\bmod", "\\mathchoice{\\mskip1mu}{\\mskip1mu}{\\mskip5mu}{\\mskip5mu}\\mathbin{\\rm mod}\\mathchoice{\\mskip1mu}{\\mskip1mu}{\\mskip5mu}{\\mskip5mu}");
d("\\pod", "\\allowbreak\\mathchoice{\\mkern18mu}{\\mkern8mu}{\\mkern8mu}{\\mkern8mu}(#1)");
d("\\pmod", "\\pod{{\\rm mod}\\mkern6mu#1}");
d("\\mod", "\\allowbreak\\mathchoice{\\mkern18mu}{\\mkern12mu}{\\mkern12mu}{\\mkern12mu}{\\rm mod}\\,\\,#1");
d("\\newline", "\\\\\\relax");
d("\\TeX", "\\textrm{\\html@mathml{T\\kern-.1667em\\raisebox{-.5ex}{E}\\kern-.125emX}{TeX}}");
var qa = B(I0["Main-Regular"][84][1] - 0.7 * I0["Main-Regular"][65][1]);
d("\\LaTeX", "\\textrm{\\html@mathml{" + ("L\\kern-.36em\\raisebox{" + qa + "}{\\scriptstyle A}") + "\\kern-.15em\\TeX}{LaTeX}}");
d("\\KaTeX", "\\textrm{\\html@mathml{" + ("K\\kern-.17em\\raisebox{" + qa + "}{\\scriptstyle A}") + "\\kern-.15em\\TeX}{KaTeX}}");
d("\\hspace", "\\@ifstar\\@hspacer\\@hspace");
d("\\@hspace", "\\hskip #1\\relax");
d("\\@hspacer", "\\rule{0pt}{0pt}\\hskip #1\\relax");
d("\\ordinarycolon", ":");
d("\\vcentcolon", "\\mathrel{\\mathop\\ordinarycolon}");
d("\\dblcolon", '\\html@mathml{\\mathrel{\\vcentcolon\\mathrel{\\mkern-.9mu}\\vcentcolon}}{\\mathop{\\char"2237}}');
d("\\coloneqq", '\\html@mathml{\\mathrel{\\vcentcolon\\mathrel{\\mkern-1.2mu}=}}{\\mathop{\\char"2254}}');
d("\\Coloneqq", '\\html@mathml{\\mathrel{\\dblcolon\\mathrel{\\mkern-1.2mu}=}}{\\mathop{\\char"2237\\char"3d}}');
d("\\coloneq", '\\html@mathml{\\mathrel{\\vcentcolon\\mathrel{\\mkern-1.2mu}\\mathrel{-}}}{\\mathop{\\char"3a\\char"2212}}');
d("\\Coloneq", '\\html@mathml{\\mathrel{\\dblcolon\\mathrel{\\mkern-1.2mu}\\mathrel{-}}}{\\mathop{\\char"2237\\char"2212}}');
d("\\eqqcolon", '\\html@mathml{\\mathrel{=\\mathrel{\\mkern-1.2mu}\\vcentcolon}}{\\mathop{\\char"2255}}');
d("\\Eqqcolon", '\\html@mathml{\\mathrel{=\\mathrel{\\mkern-1.2mu}\\dblcolon}}{\\mathop{\\char"3d\\char"2237}}');
d("\\eqcolon", '\\html@mathml{\\mathrel{\\mathrel{-}\\mathrel{\\mkern-1.2mu}\\vcentcolon}}{\\mathop{\\char"2239}}');
d("\\Eqcolon", '\\html@mathml{\\mathrel{\\mathrel{-}\\mathrel{\\mkern-1.2mu}\\dblcolon}}{\\mathop{\\char"2212\\char"2237}}');
d("\\colonapprox", '\\html@mathml{\\mathrel{\\vcentcolon\\mathrel{\\mkern-1.2mu}\\approx}}{\\mathop{\\char"3a\\char"2248}}');
d("\\Colonapprox", '\\html@mathml{\\mathrel{\\dblcolon\\mathrel{\\mkern-1.2mu}\\approx}}{\\mathop{\\char"2237\\char"2248}}');
d("\\colonsim", '\\html@mathml{\\mathrel{\\vcentcolon\\mathrel{\\mkern-1.2mu}\\sim}}{\\mathop{\\char"3a\\char"223c}}');
d("\\Colonsim", '\\html@mathml{\\mathrel{\\dblcolon\\mathrel{\\mkern-1.2mu}\\sim}}{\\mathop{\\char"2237\\char"223c}}');
d("∷", "\\dblcolon");
d("∹", "\\eqcolon");
d("≔", "\\coloneqq");
d("≕", "\\eqqcolon");
d("⩴", "\\Coloneqq");
d("\\ratio", "\\vcentcolon");
d("\\coloncolon", "\\dblcolon");
d("\\colonequals", "\\coloneqq");
d("\\coloncolonequals", "\\Coloneqq");
d("\\equalscolon", "\\eqqcolon");
d("\\equalscoloncolon", "\\Eqqcolon");
d("\\colonminus", "\\coloneq");
d("\\coloncolonminus", "\\Coloneq");
d("\\minuscolon", "\\eqcolon");
d("\\minuscoloncolon", "\\Eqcolon");
d("\\coloncolonapprox", "\\Colonapprox");
d("\\coloncolonsim", "\\Colonsim");
d("\\simcolon", "\\mathrel{\\sim\\mathrel{\\mkern-1.2mu}\\vcentcolon}");
d("\\simcoloncolon", "\\mathrel{\\sim\\mathrel{\\mkern-1.2mu}\\dblcolon}");
d("\\approxcolon", "\\mathrel{\\approx\\mathrel{\\mkern-1.2mu}\\vcentcolon}");
d("\\approxcoloncolon", "\\mathrel{\\approx\\mathrel{\\mkern-1.2mu}\\dblcolon}");
d("\\notni", "\\html@mathml{\\not\\ni}{\\mathrel{\\char`∌}}");
d("\\limsup", "\\DOTSB\\operatorname*{lim\\,sup}");
d("\\liminf", "\\DOTSB\\operatorname*{lim\\,inf}");
d("\\injlim", "\\DOTSB\\operatorname*{inj\\,lim}");
d("\\projlim", "\\DOTSB\\operatorname*{proj\\,lim}");
d("\\varlimsup", "\\DOTSB\\operatorname*{\\overline{lim}}");
d("\\varliminf", "\\DOTSB\\operatorname*{\\underline{lim}}");
d("\\varinjlim", "\\DOTSB\\operatorname*{\\underrightarrow{lim}}");
d("\\varprojlim", "\\DOTSB\\operatorname*{\\underleftarrow{lim}}");
d("\\gvertneqq", "\\html@mathml{\\@gvertneqq}{≩}");
d("\\lvertneqq", "\\html@mathml{\\@lvertneqq}{≨}");
d("\\ngeqq", "\\html@mathml{\\@ngeqq}{≱}");
d("\\ngeqslant", "\\html@mathml{\\@ngeqslant}{≱}");
d("\\nleqq", "\\html@mathml{\\@nleqq}{≰}");
d("\\nleqslant", "\\html@mathml{\\@nleqslant}{≰}");
d("\\nshortmid", "\\html@mathml{\\@nshortmid}{∤}");
d("\\nshortparallel", "\\html@mathml{\\@nshortparallel}{∦}");
d("\\nsubseteqq", "\\html@mathml{\\@nsubseteqq}{⊈}");
d("\\nsupseteqq", "\\html@mathml{\\@nsupseteqq}{⊉}");
d("\\varsubsetneq", "\\html@mathml{\\@varsubsetneq}{⊊}");
d("\\varsubsetneqq", "\\html@mathml{\\@varsubsetneqq}{⫋}");
d("\\varsupsetneq", "\\html@mathml{\\@varsupsetneq}{⊋}");
d("\\varsupsetneqq", "\\html@mathml{\\@varsupsetneqq}{⫌}");
d("\\imath", "\\html@mathml{\\@imath}{ı}");
d("\\jmath", "\\html@mathml{\\@jmath}{ȷ}");
d("\\llbracket", "\\html@mathml{\\mathopen{[\\mkern-3.2mu[}}{\\mathopen{\\char`⟦}}");
d("\\rrbracket", "\\html@mathml{\\mathclose{]\\mkern-3.2mu]}}{\\mathclose{\\char`⟧}}");
d("⟦", "\\llbracket");
d("⟧", "\\rrbracket");
d("\\lBrace", "\\html@mathml{\\mathopen{\\{\\mkern-3.2mu[}}{\\mathopen{\\char`⦃}}");
d("\\rBrace", "\\html@mathml{\\mathclose{]\\mkern-3.2mu\\}}}{\\mathclose{\\char`⦄}}");
d("⦃", "\\lBrace");
d("⦄", "\\rBrace");
d("\\minuso", "\\mathbin{\\html@mathml{{\\mathrlap{\\mathchoice{\\kern{0.145em}}{\\kern{0.145em}}{\\kern{0.1015em}}{\\kern{0.0725em}}\\circ}{-}}}{\\char`⦵}}");
d("⦵", "\\minuso");
d("\\darr", "\\downarrow");
d("\\dArr", "\\Downarrow");
d("\\Darr", "\\Downarrow");
d("\\lang", "\\langle");
d("\\rang", "\\rangle");
d("\\uarr", "\\uparrow");
d("\\uArr", "\\Uparrow");
d("\\Uarr", "\\Uparrow");
d("\\N", "\\mathbb{N}");
d("\\R", "\\mathbb{R}");
d("\\Z", "\\mathbb{Z}");
d("\\alef", "\\aleph");
d("\\alefsym", "\\aleph");
d("\\Alpha", "\\mathrm{A}");
d("\\Beta", "\\mathrm{B}");
d("\\bull", "\\bullet");
d("\\Chi", "\\mathrm{X}");
d("\\clubs", "\\clubsuit");
d("\\cnums", "\\mathbb{C}");
d("\\Complex", "\\mathbb{C}");
d("\\Dagger", "\\ddagger");
d("\\diamonds", "\\diamondsuit");
d("\\empty", "\\emptyset");
d("\\Epsilon", "\\mathrm{E}");
d("\\Eta", "\\mathrm{H}");
d("\\exist", "\\exists");
d("\\harr", "\\leftrightarrow");
d("\\hArr", "\\Leftrightarrow");
d("\\Harr", "\\Leftrightarrow");
d("\\hearts", "\\heartsuit");
d("\\image", "\\Im");
d("\\infin", "\\infty");
d("\\Iota", "\\mathrm{I}");
d("\\isin", "\\in");
d("\\Kappa", "\\mathrm{K}");
d("\\larr", "\\leftarrow");
d("\\lArr", "\\Leftarrow");
d("\\Larr", "\\Leftarrow");
d("\\lrarr", "\\leftrightarrow");
d("\\lrArr", "\\Leftrightarrow");
d("\\Lrarr", "\\Leftrightarrow");
d("\\Mu", "\\mathrm{M}");
d("\\natnums", "\\mathbb{N}");
d("\\Nu", "\\mathrm{N}");
d("\\Omicron", "\\mathrm{O}");
d("\\plusmn", "\\pm");
d("\\rarr", "\\rightarrow");
d("\\rArr", "\\Rightarrow");
d("\\Rarr", "\\Rightarrow");
d("\\real", "\\Re");
d("\\reals", "\\mathbb{R}");
d("\\Reals", "\\mathbb{R}");
d("\\Rho", "\\mathrm{P}");
d("\\sdot", "\\cdot");
d("\\sect", "\\S");
d("\\spades", "\\spadesuit");
d("\\sub", "\\subset");
d("\\sube", "\\subseteq");
d("\\supe", "\\supseteq");
d("\\Tau", "\\mathrm{T}");
d("\\thetasym", "\\vartheta");
d("\\weierp", "\\wp");
d("\\Zeta", "\\mathrm{Z}");
d("\\argmin", "\\DOTSB\\operatorname*{arg\\,min}");
d("\\argmax", "\\DOTSB\\operatorname*{arg\\,max}");
d("\\plim", "\\DOTSB\\mathop{\\operatorname{plim}}\\limits");
d("\\bra", "\\mathinner{\\langle{#1}|}");
d("\\ket", "\\mathinner{|{#1}\\rangle}");
d("\\braket", "\\mathinner{\\langle{#1}\\rangle}");
d("\\Bra", "\\left\\langle#1\\right|");
d("\\Ket", "\\left|#1\\right\\rangle");
var _a = (r) => (e) => {
  var t = e.consumeArg().tokens, a = e.consumeArg().tokens, n = e.consumeArg().tokens, s = e.consumeArg().tokens, u = e.macros.get("|"), o = e.macros.get("\\|");
  e.macros.beginGroup();
  var h = (b) => (k) => {
    r && (k.macros.set("|", u), n.length && k.macros.set("\\|", o));
    var S = b;
    if (!b && n.length) {
      var _ = k.future();
      _.text === "|" && (k.popToken(), S = !0);
    }
    return {
      tokens: S ? n : a,
      numArgs: 0
    };
  };
  e.macros.set("|", h(!1)), n.length && e.macros.set("\\|", h(!0));
  var m = e.consumeArg().tokens, v = e.expandTokens([
    ...s,
    ...m,
    ...t
    // reversed
  ]);
  return e.macros.endGroup(), {
    tokens: v.reverse(),
    numArgs: 0
  };
};
d("\\bra@ket", _a(!1));
d("\\bra@set", _a(!0));
d("\\Braket", "\\bra@ket{\\left\\langle}{\\,\\middle\\vert\\,}{\\,\\middle\\vert\\,}{\\right\\rangle}");
d("\\Set", "\\bra@set{\\left\\{\\:}{\\;\\middle\\vert\\;}{\\;\\middle\\Vert\\;}{\\:\\right\\}}");
d("\\set", "\\bra@set{\\{\\,}{\\mid}{}{\\,\\}}");
d("\\angln", "{\\angl n}");
d("\\blue", "\\textcolor{##6495ed}{#1}");
d("\\orange", "\\textcolor{##ffa500}{#1}");
d("\\pink", "\\textcolor{##ff00af}{#1}");
d("\\red", "\\textcolor{##df0030}{#1}");
d("\\green", "\\textcolor{##28ae7b}{#1}");
d("\\gray", "\\textcolor{gray}{#1}");
d("\\purple", "\\textcolor{##9d38bd}{#1}");
d("\\blueA", "\\textcolor{##ccfaff}{#1}");
d("\\blueB", "\\textcolor{##80f6ff}{#1}");
d("\\blueC", "\\textcolor{##63d9ea}{#1}");
d("\\blueD", "\\textcolor{##11accd}{#1}");
d("\\blueE", "\\textcolor{##0c7f99}{#1}");
d("\\tealA", "\\textcolor{##94fff5}{#1}");
d("\\tealB", "\\textcolor{##26edd5}{#1}");
d("\\tealC", "\\textcolor{##01d1c1}{#1}");
d("\\tealD", "\\textcolor{##01a995}{#1}");
d("\\tealE", "\\textcolor{##208170}{#1}");
d("\\greenA", "\\textcolor{##b6ffb0}{#1}");
d("\\greenB", "\\textcolor{##8af281}{#1}");
d("\\greenC", "\\textcolor{##74cf70}{#1}");
d("\\greenD", "\\textcolor{##1fab54}{#1}");
d("\\greenE", "\\textcolor{##0d923f}{#1}");
d("\\goldA", "\\textcolor{##ffd0a9}{#1}");
d("\\goldB", "\\textcolor{##ffbb71}{#1}");
d("\\goldC", "\\textcolor{##ff9c39}{#1}");
d("\\goldD", "\\textcolor{##e07d10}{#1}");
d("\\goldE", "\\textcolor{##a75a05}{#1}");
d("\\redA", "\\textcolor{##fca9a9}{#1}");
d("\\redB", "\\textcolor{##ff8482}{#1}");
d("\\redC", "\\textcolor{##f9685d}{#1}");
d("\\redD", "\\textcolor{##e84d39}{#1}");
d("\\redE", "\\textcolor{##bc2612}{#1}");
d("\\maroonA", "\\textcolor{##ffbde0}{#1}");
d("\\maroonB", "\\textcolor{##ff92c6}{#1}");
d("\\maroonC", "\\textcolor{##ed5fa6}{#1}");
d("\\maroonD", "\\textcolor{##ca337c}{#1}");
d("\\maroonE", "\\textcolor{##9e034e}{#1}");
d("\\purpleA", "\\textcolor{##ddd7ff}{#1}");
d("\\purpleB", "\\textcolor{##c6b9fc}{#1}");
d("\\purpleC", "\\textcolor{##aa87ff}{#1}");
d("\\purpleD", "\\textcolor{##7854ab}{#1}");
d("\\purpleE", "\\textcolor{##543b78}{#1}");
d("\\mintA", "\\textcolor{##f5f9e8}{#1}");
d("\\mintB", "\\textcolor{##edf2df}{#1}");
d("\\mintC", "\\textcolor{##e0e5cc}{#1}");
d("\\grayA", "\\textcolor{##f6f7f7}{#1}");
d("\\grayB", "\\textcolor{##f0f1f2}{#1}");
d("\\grayC", "\\textcolor{##e3e5e6}{#1}");
d("\\grayD", "\\textcolor{##d6d8da}{#1}");
d("\\grayE", "\\textcolor{##babec2}{#1}");
d("\\grayF", "\\textcolor{##888d93}{#1}");
d("\\grayG", "\\textcolor{##626569}{#1}");
d("\\grayH", "\\textcolor{##3b3e40}{#1}");
d("\\grayI", "\\textcolor{##21242c}{#1}");
d("\\kaBlue", "\\textcolor{##314453}{#1}");
d("\\kaGreen", "\\textcolor{##71B307}{#1}");
var Oa = {
  "^": !0,
  // Parser.js
  _: !0,
  // Parser.js
  "\\limits": !0,
  // Parser.js
  "\\nolimits": !0
  // Parser.js
};
class gs {
  constructor(e, t, a) {
    this.settings = void 0, this.expansionCount = void 0, this.lexer = void 0, this.macros = void 0, this.stack = void 0, this.mode = void 0, this.settings = t, this.expansionCount = 0, this.feed(e), this.macros = new fs(ps, t.macros), this.mode = a, this.stack = [];
  }
  /**
   * Feed a new input string to the same MacroExpander
   * (with existing macros etc.).
   */
  feed(e) {
    this.lexer = new Pr(e, this.settings);
  }
  /**
   * Switches between "text" and "math" modes.
   */
  switchMode(e) {
    this.mode = e;
  }
  /**
   * Start a new group nesting within all namespaces.
   */
  beginGroup() {
    this.macros.beginGroup();
  }
  /**
   * End current group nesting within all namespaces.
   */
  endGroup() {
    this.macros.endGroup();
  }
  /**
   * Ends all currently nested groups (if any), restoring values before the
   * groups began.  Useful in case of an error in the middle of parsing.
   */
  endGroups() {
    this.macros.endGroups();
  }
  /**
   * Returns the topmost token on the stack, without expanding it.
   * Similar in behavior to TeX's `\futurelet`.
   */
  future() {
    return this.stack.length === 0 && this.pushToken(this.lexer.lex()), this.stack[this.stack.length - 1];
  }
  /**
   * Remove and return the next unexpanded token.
   */
  popToken() {
    return this.future(), this.stack.pop();
  }
  /**
   * Add a given token to the token stack.  In particular, this get be used
   * to put back a token returned from one of the other methods.
   */
  pushToken(e) {
    this.stack.push(e);
  }
  /**
   * Append an array of tokens to the token stack.
   */
  pushTokens(e) {
    this.stack.push(...e);
  }
  /**
   * Find an macro argument without expanding tokens and append the array of
   * tokens to the token stack. Uses Token as a container for the result.
   */
  scanArgument(e) {
    var t, a, n;
    if (e) {
      if (this.consumeSpaces(), this.future().text !== "[")
        return null;
      t = this.popToken(), {
        tokens: n,
        end: a
      } = this.consumeArg(["]"]);
    } else
      ({
        tokens: n,
        start: t,
        end: a
      } = this.consumeArg());
    return this.pushToken(new T0("EOF", a.loc)), this.pushTokens(n), new T0("", y0.range(t, a));
  }
  /**
   * Consume all following space tokens, without expansion.
   */
  consumeSpaces() {
    for (; ; ) {
      var e = this.future();
      if (e.text === " ")
        this.stack.pop();
      else
        break;
    }
  }
  /**
   * Consume an argument from the token stream, and return the resulting array
   * of tokens and start/end token.
   */
  consumeArg(e) {
    var t = [], a = e && e.length > 0;
    a || this.consumeSpaces();
    var n = this.future(), s, u = 0, o = 0;
    do {
      if (s = this.popToken(), t.push(s), s.text === "{")
        ++u;
      else if (s.text === "}") {
        if (--u, u === -1)
          throw new E("Extra }", s);
      } else if (s.text === "EOF")
        throw new E("Unexpected end of input in a macro argument, expected '" + (e && a ? e[o] : "}") + "'", s);
      if (e && a)
        if ((u === 0 || u === 1 && e[o] === "{") && s.text === e[o]) {
          if (++o, o === e.length) {
            t.splice(-o, o);
            break;
          }
        } else
          o = 0;
    } while (u !== 0 || a);
    return n.text === "{" && t[t.length - 1].text === "}" && (t.pop(), t.shift()), t.reverse(), {
      tokens: t,
      start: n,
      end: s
    };
  }
  /**
   * Consume the specified number of (delimited) arguments from the token
   * stream and return the resulting array of arguments.
   */
  consumeArgs(e, t) {
    if (t) {
      if (t.length !== e + 1)
        throw new E("The length of delimiters doesn't match the number of args!");
      for (var a = t[0], n = 0; n < a.length; n++) {
        var s = this.popToken();
        if (a[n] !== s.text)
          throw new E("Use of the macro doesn't match its definition", s);
      }
    }
    for (var u = [], o = 0; o < e; o++)
      u.push(this.consumeArg(t && t[o + 1]).tokens);
    return u;
  }
  /**
   * Increment `expansionCount` by the specified amount.
   * Throw an error if it exceeds `maxExpand`.
   */
  countExpansion(e) {
    if (this.expansionCount += e, this.expansionCount > this.settings.maxExpand)
      throw new E("Too many expansions: infinite loop or need to increase maxExpand setting");
  }
  /**
   * Expand the next token only once if possible.
   *
   * If the token is expanded, the resulting tokens will be pushed onto
   * the stack in reverse order, and the number of such tokens will be
   * returned.  This number might be zero or positive.
   *
   * If not, the return value is `false`, and the next token remains at the
   * top of the stack.
   *
   * In either case, the next token will be on the top of the stack,
   * or the stack will be empty (in case of empty expansion
   * and no other tokens).
   *
   * Used to implement `expandAfterFuture` and `expandNextToken`.
   *
   * If expandableOnly, only expandable tokens are expanded and
   * an undefined control sequence results in an error.
   */
  expandOnce(e) {
    var t = this.popToken(), a = t.text, n = t.noexpand ? null : this._getExpansion(a);
    if (n == null || e && n.unexpandable) {
      if (e && n == null && a[0] === "\\" && !this.isDefined(a))
        throw new E("Undefined control sequence: " + a);
      return this.pushToken(t), !1;
    }
    this.countExpansion(1);
    var s = n.tokens, u = this.consumeArgs(n.numArgs, n.delimiters);
    if (n.numArgs) {
      s = s.slice();
      for (var o = s.length - 1; o >= 0; --o) {
        var h = s[o];
        if (h.text === "#") {
          if (o === 0)
            throw new E("Incomplete placeholder at end of macro body", h);
          if (h = s[--o], h.text === "#")
            s.splice(o + 1, 1);
          else if (/^[1-9]$/.test(h.text))
            s.splice(o, 2, ...u[+h.text - 1]);
          else
            throw new E("Not a valid argument number", h);
        }
      }
    }
    return this.pushTokens(s), s.length;
  }
  /**
   * Expand the next token only once (if possible), and return the resulting
   * top token on the stack (without removing anything from the stack).
   * Similar in behavior to TeX's `\expandafter\futurelet`.
   * Equivalent to expandOnce() followed by future().
   */
  expandAfterFuture() {
    return this.expandOnce(), this.future();
  }
  /**
   * Recursively expand first token, then return first non-expandable token.
   */
  expandNextToken() {
    for (; ; )
      if (this.expandOnce() === !1) {
        var e = this.stack.pop();
        return e.treatAsRelax && (e.text = "\\relax"), e;
      }
  }
  /**
   * Fully expand the given macro name and return the resulting list of
   * tokens, or return `undefined` if no such macro is defined.
   */
  expandMacro(e) {
    return this.macros.has(e) ? this.expandTokens([new T0(e)]) : void 0;
  }
  /**
   * Fully expand the given token stream and return the resulting list of
   * tokens.  Note that the input tokens are in reverse order, but the
   * output tokens are in forward order.
   */
  expandTokens(e) {
    var t = [], a = this.stack.length;
    for (this.pushTokens(e); this.stack.length > a; )
      if (this.expandOnce(!0) === !1) {
        var n = this.stack.pop();
        n.treatAsRelax && (n.noexpand = !1, n.treatAsRelax = !1), t.push(n);
      }
    return this.countExpansion(t.length), t;
  }
  /**
   * Fully expand the given macro name and return the result as a string,
   * or return `undefined` if no such macro is defined.
   */
  expandMacroAsText(e) {
    var t = this.expandMacro(e);
    return t && t.map((a) => a.text).join("");
  }
  /**
   * Returns the expanded macro as a reversed array of tokens and a macro
   * argument count.  Or returns `null` if no such macro.
   */
  _getExpansion(e) {
    var t = this.macros.get(e);
    if (t == null)
      return t;
    if (e.length === 1) {
      var a = this.lexer.catcodes[e];
      if (a != null && a !== 13)
        return;
    }
    var n = typeof t == "function" ? t(this) : t;
    if (typeof n == "string") {
      var s = 0;
      if (n.includes("#"))
        for (var u = n.replace(/##/g, ""); u.includes("#" + (s + 1)); )
          ++s;
      for (var o = new Pr(n, this.settings), h = [], m = o.lex(); m.text !== "EOF"; )
        h.push(m), m = o.lex();
      h.reverse();
      var v = {
        tokens: h,
        numArgs: s
      };
      return v;
    }
    return n;
  }
  /**
   * Determine whether a command is currently "defined" (has some
   * functionality), meaning that it's a macro (in the current group),
   * a function, a symbol, or one of the special commands listed in
   * `implicitCommands`.
   */
  isDefined(e) {
    return this.macros.has(e) || J0.hasOwnProperty(e) || Q.math.hasOwnProperty(e) || Q.text.hasOwnProperty(e) || Oa.hasOwnProperty(e);
  }
  /**
   * Determine whether a command is expandable.
   */
  isExpandable(e) {
    var t = this.macros.get(e);
    return t != null ? typeof t == "string" || typeof t == "function" || !t.unexpandable : J0.hasOwnProperty(e) && !J0[e].primitive;
  }
}
var jr = /^[₊₋₌₍₎₀₁₂₃₄₅₆₇₈₉ₐₑₕᵢⱼₖₗₘₙₒₚᵣₛₜᵤᵥₓᵦᵧᵨᵩᵪ]/, He = Object.freeze({
  "₊": "+",
  "₋": "-",
  "₌": "=",
  "₍": "(",
  "₎": ")",
  "₀": "0",
  "₁": "1",
  "₂": "2",
  "₃": "3",
  "₄": "4",
  "₅": "5",
  "₆": "6",
  "₇": "7",
  "₈": "8",
  "₉": "9",
  "ₐ": "a",
  "ₑ": "e",
  "ₕ": "h",
  "ᵢ": "i",
  "ⱼ": "j",
  "ₖ": "k",
  "ₗ": "l",
  "ₘ": "m",
  "ₙ": "n",
  "ₒ": "o",
  "ₚ": "p",
  "ᵣ": "r",
  "ₛ": "s",
  "ₜ": "t",
  "ᵤ": "u",
  "ᵥ": "v",
  "ₓ": "x",
  "ᵦ": "β",
  "ᵧ": "γ",
  "ᵨ": "ρ",
  "ᵩ": "ϕ",
  "ᵪ": "χ",
  "⁺": "+",
  "⁻": "-",
  "⁼": "=",
  "⁽": "(",
  "⁾": ")",
  "⁰": "0",
  "¹": "1",
  "²": "2",
  "³": "3",
  "⁴": "4",
  "⁵": "5",
  "⁶": "6",
  "⁷": "7",
  "⁸": "8",
  "⁹": "9",
  "ᴬ": "A",
  "ᴮ": "B",
  "ᴰ": "D",
  "ᴱ": "E",
  "ᴳ": "G",
  "ᴴ": "H",
  "ᴵ": "I",
  "ᴶ": "J",
  "ᴷ": "K",
  "ᴸ": "L",
  "ᴹ": "M",
  "ᴺ": "N",
  "ᴼ": "O",
  "ᴾ": "P",
  "ᴿ": "R",
  "ᵀ": "T",
  "ᵁ": "U",
  "ⱽ": "V",
  "ᵂ": "W",
  "ᵃ": "a",
  "ᵇ": "b",
  "ᶜ": "c",
  "ᵈ": "d",
  "ᵉ": "e",
  "ᶠ": "f",
  "ᵍ": "g",
  ʰ: "h",
  "ⁱ": "i",
  ʲ: "j",
  "ᵏ": "k",
  ˡ: "l",
  "ᵐ": "m",
  ⁿ: "n",
  "ᵒ": "o",
  "ᵖ": "p",
  ʳ: "r",
  ˢ: "s",
  "ᵗ": "t",
  "ᵘ": "u",
  "ᵛ": "v",
  ʷ: "w",
  ˣ: "x",
  ʸ: "y",
  "ᶻ": "z",
  "ᵝ": "β",
  "ᵞ": "γ",
  "ᵟ": "δ",
  "ᵠ": "ϕ",
  "ᵡ": "χ",
  "ᶿ": "θ"
}), xt = {
  "́": {
    text: "\\'",
    math: "\\acute"
  },
  "̀": {
    text: "\\`",
    math: "\\grave"
  },
  "̈": {
    text: '\\"',
    math: "\\ddot"
  },
  "̃": {
    text: "\\~",
    math: "\\tilde"
  },
  "̄": {
    text: "\\=",
    math: "\\bar"
  },
  "̆": {
    text: "\\u",
    math: "\\breve"
  },
  "̌": {
    text: "\\v",
    math: "\\check"
  },
  "̂": {
    text: "\\^",
    math: "\\hat"
  },
  "̇": {
    text: "\\.",
    math: "\\dot"
  },
  "̊": {
    text: "\\r",
    math: "\\mathring"
  },
  "̋": {
    text: "\\H"
  },
  "̧": {
    text: "\\c"
  }
}, Xr = {
  á: "á",
  à: "à",
  ä: "ä",
  ǟ: "ǟ",
  ã: "ã",
  ā: "ā",
  ă: "ă",
  ắ: "ắ",
  ằ: "ằ",
  ẵ: "ẵ",
  ǎ: "ǎ",
  â: "â",
  ấ: "ấ",
  ầ: "ầ",
  ẫ: "ẫ",
  ȧ: "ȧ",
  ǡ: "ǡ",
  å: "å",
  ǻ: "ǻ",
  ḃ: "ḃ",
  ć: "ć",
  ḉ: "ḉ",
  č: "č",
  ĉ: "ĉ",
  ċ: "ċ",
  ç: "ç",
  ď: "ď",
  ḋ: "ḋ",
  ḑ: "ḑ",
  é: "é",
  è: "è",
  ë: "ë",
  ẽ: "ẽ",
  ē: "ē",
  ḗ: "ḗ",
  ḕ: "ḕ",
  ĕ: "ĕ",
  ḝ: "ḝ",
  ě: "ě",
  ê: "ê",
  ế: "ế",
  ề: "ề",
  ễ: "ễ",
  ė: "ė",
  ȩ: "ȩ",
  ḟ: "ḟ",
  ǵ: "ǵ",
  ḡ: "ḡ",
  ğ: "ğ",
  ǧ: "ǧ",
  ĝ: "ĝ",
  ġ: "ġ",
  ģ: "ģ",
  ḧ: "ḧ",
  ȟ: "ȟ",
  ĥ: "ĥ",
  ḣ: "ḣ",
  ḩ: "ḩ",
  í: "í",
  ì: "ì",
  ï: "ï",
  ḯ: "ḯ",
  ĩ: "ĩ",
  ī: "ī",
  ĭ: "ĭ",
  ǐ: "ǐ",
  î: "î",
  ǰ: "ǰ",
  ĵ: "ĵ",
  ḱ: "ḱ",
  ǩ: "ǩ",
  ķ: "ķ",
  ĺ: "ĺ",
  ľ: "ľ",
  ļ: "ļ",
  ḿ: "ḿ",
  ṁ: "ṁ",
  ń: "ń",
  ǹ: "ǹ",
  ñ: "ñ",
  ň: "ň",
  ṅ: "ṅ",
  ņ: "ņ",
  ó: "ó",
  ò: "ò",
  ö: "ö",
  ȫ: "ȫ",
  õ: "õ",
  ṍ: "ṍ",
  ṏ: "ṏ",
  ȭ: "ȭ",
  ō: "ō",
  ṓ: "ṓ",
  ṑ: "ṑ",
  ŏ: "ŏ",
  ǒ: "ǒ",
  ô: "ô",
  ố: "ố",
  ồ: "ồ",
  ỗ: "ỗ",
  ȯ: "ȯ",
  ȱ: "ȱ",
  ő: "ő",
  ṕ: "ṕ",
  ṗ: "ṗ",
  ŕ: "ŕ",
  ř: "ř",
  ṙ: "ṙ",
  ŗ: "ŗ",
  ś: "ś",
  ṥ: "ṥ",
  š: "š",
  ṧ: "ṧ",
  ŝ: "ŝ",
  ṡ: "ṡ",
  ş: "ş",
  ẗ: "ẗ",
  ť: "ť",
  ṫ: "ṫ",
  ţ: "ţ",
  ú: "ú",
  ù: "ù",
  ü: "ü",
  ǘ: "ǘ",
  ǜ: "ǜ",
  ǖ: "ǖ",
  ǚ: "ǚ",
  ũ: "ũ",
  ṹ: "ṹ",
  ū: "ū",
  ṻ: "ṻ",
  ŭ: "ŭ",
  ǔ: "ǔ",
  û: "û",
  ů: "ů",
  ű: "ű",
  ṽ: "ṽ",
  ẃ: "ẃ",
  ẁ: "ẁ",
  ẅ: "ẅ",
  ŵ: "ŵ",
  ẇ: "ẇ",
  ẘ: "ẘ",
  ẍ: "ẍ",
  ẋ: "ẋ",
  ý: "ý",
  ỳ: "ỳ",
  ÿ: "ÿ",
  ỹ: "ỹ",
  ȳ: "ȳ",
  ŷ: "ŷ",
  ẏ: "ẏ",
  ẙ: "ẙ",
  ź: "ź",
  ž: "ž",
  ẑ: "ẑ",
  ż: "ż",
  Á: "Á",
  À: "À",
  Ä: "Ä",
  Ǟ: "Ǟ",
  Ã: "Ã",
  Ā: "Ā",
  Ă: "Ă",
  Ắ: "Ắ",
  Ằ: "Ằ",
  Ẵ: "Ẵ",
  Ǎ: "Ǎ",
  Â: "Â",
  Ấ: "Ấ",
  Ầ: "Ầ",
  Ẫ: "Ẫ",
  Ȧ: "Ȧ",
  Ǡ: "Ǡ",
  Å: "Å",
  Ǻ: "Ǻ",
  Ḃ: "Ḃ",
  Ć: "Ć",
  Ḉ: "Ḉ",
  Č: "Č",
  Ĉ: "Ĉ",
  Ċ: "Ċ",
  Ç: "Ç",
  Ď: "Ď",
  Ḋ: "Ḋ",
  Ḑ: "Ḑ",
  É: "É",
  È: "È",
  Ë: "Ë",
  Ẽ: "Ẽ",
  Ē: "Ē",
  Ḗ: "Ḗ",
  Ḕ: "Ḕ",
  Ĕ: "Ĕ",
  Ḝ: "Ḝ",
  Ě: "Ě",
  Ê: "Ê",
  Ế: "Ế",
  Ề: "Ề",
  Ễ: "Ễ",
  Ė: "Ė",
  Ȩ: "Ȩ",
  Ḟ: "Ḟ",
  Ǵ: "Ǵ",
  Ḡ: "Ḡ",
  Ğ: "Ğ",
  Ǧ: "Ǧ",
  Ĝ: "Ĝ",
  Ġ: "Ġ",
  Ģ: "Ģ",
  Ḧ: "Ḧ",
  Ȟ: "Ȟ",
  Ĥ: "Ĥ",
  Ḣ: "Ḣ",
  Ḩ: "Ḩ",
  Í: "Í",
  Ì: "Ì",
  Ï: "Ï",
  Ḯ: "Ḯ",
  Ĩ: "Ĩ",
  Ī: "Ī",
  Ĭ: "Ĭ",
  Ǐ: "Ǐ",
  Î: "Î",
  İ: "İ",
  Ĵ: "Ĵ",
  Ḱ: "Ḱ",
  Ǩ: "Ǩ",
  Ķ: "Ķ",
  Ĺ: "Ĺ",
  Ľ: "Ľ",
  Ļ: "Ļ",
  Ḿ: "Ḿ",
  Ṁ: "Ṁ",
  Ń: "Ń",
  Ǹ: "Ǹ",
  Ñ: "Ñ",
  Ň: "Ň",
  Ṅ: "Ṅ",
  Ņ: "Ņ",
  Ó: "Ó",
  Ò: "Ò",
  Ö: "Ö",
  Ȫ: "Ȫ",
  Õ: "Õ",
  Ṍ: "Ṍ",
  Ṏ: "Ṏ",
  Ȭ: "Ȭ",
  Ō: "Ō",
  Ṓ: "Ṓ",
  Ṑ: "Ṑ",
  Ŏ: "Ŏ",
  Ǒ: "Ǒ",
  Ô: "Ô",
  Ố: "Ố",
  Ồ: "Ồ",
  Ỗ: "Ỗ",
  Ȯ: "Ȯ",
  Ȱ: "Ȱ",
  Ő: "Ő",
  Ṕ: "Ṕ",
  Ṗ: "Ṗ",
  Ŕ: "Ŕ",
  Ř: "Ř",
  Ṙ: "Ṙ",
  Ŗ: "Ŗ",
  Ś: "Ś",
  Ṥ: "Ṥ",
  Š: "Š",
  Ṧ: "Ṧ",
  Ŝ: "Ŝ",
  Ṡ: "Ṡ",
  Ş: "Ş",
  Ť: "Ť",
  Ṫ: "Ṫ",
  Ţ: "Ţ",
  Ú: "Ú",
  Ù: "Ù",
  Ü: "Ü",
  Ǘ: "Ǘ",
  Ǜ: "Ǜ",
  Ǖ: "Ǖ",
  Ǚ: "Ǚ",
  Ũ: "Ũ",
  Ṹ: "Ṹ",
  Ū: "Ū",
  Ṻ: "Ṻ",
  Ŭ: "Ŭ",
  Ǔ: "Ǔ",
  Û: "Û",
  Ů: "Ů",
  Ű: "Ű",
  Ṽ: "Ṽ",
  Ẃ: "Ẃ",
  Ẁ: "Ẁ",
  Ẅ: "Ẅ",
  Ŵ: "Ŵ",
  Ẇ: "Ẇ",
  Ẍ: "Ẍ",
  Ẋ: "Ẋ",
  Ý: "Ý",
  Ỳ: "Ỳ",
  Ÿ: "Ÿ",
  Ỹ: "Ỹ",
  Ȳ: "Ȳ",
  Ŷ: "Ŷ",
  Ẏ: "Ẏ",
  Ź: "Ź",
  Ž: "Ž",
  Ẑ: "Ẑ",
  Ż: "Ż",
  ά: "ά",
  ὰ: "ὰ",
  ᾱ: "ᾱ",
  ᾰ: "ᾰ",
  έ: "έ",
  ὲ: "ὲ",
  ή: "ή",
  ὴ: "ὴ",
  ί: "ί",
  ὶ: "ὶ",
  ϊ: "ϊ",
  ΐ: "ΐ",
  ῒ: "ῒ",
  ῑ: "ῑ",
  ῐ: "ῐ",
  ό: "ό",
  ὸ: "ὸ",
  ύ: "ύ",
  ὺ: "ὺ",
  ϋ: "ϋ",
  ΰ: "ΰ",
  ῢ: "ῢ",
  ῡ: "ῡ",
  ῠ: "ῠ",
  ώ: "ώ",
  ὼ: "ὼ",
  Ύ: "Ύ",
  Ὺ: "Ὺ",
  Ϋ: "Ϋ",
  Ῡ: "Ῡ",
  Ῠ: "Ῠ",
  Ώ: "Ώ",
  Ὼ: "Ὼ"
};
class rt {
  constructor(e, t) {
    this.mode = void 0, this.gullet = void 0, this.settings = void 0, this.leftrightDepth = void 0, this.nextToken = void 0, this.mode = "math", this.gullet = new gs(e, t, this.mode), this.settings = t, this.leftrightDepth = 0, this.nextToken = null;
  }
  /**
   * Checks a result to make sure it has the right type, and throws an
   * appropriate error otherwise.
   */
  expect(e, t) {
    if (t === void 0 && (t = !0), this.fetch().text !== e)
      throw new E("Expected '" + e + "', got '" + this.fetch().text + "'", this.fetch());
    t && this.consume();
  }
  /**
   * Discards the current lookahead token, considering it consumed.
   */
  consume() {
    this.nextToken = null;
  }
  /**
   * Return the current lookahead token, or if there isn't one (at the
   * beginning, or if the previous lookahead token was consume()d),
   * fetch the next token as the new lookahead token and return it.
   */
  fetch() {
    return this.nextToken == null && (this.nextToken = this.gullet.expandNextToken()), this.nextToken;
  }
  /**
   * Switches between "text" and "math" modes.
   */
  switchMode(e) {
    this.mode = e, this.gullet.switchMode(e);
  }
  /**
   * Main parsing function, which parses an entire input.
   */
  parse() {
    this.settings.globalGroup || this.gullet.beginGroup(), this.settings.colorIsTextColor && this.gullet.macros.set("\\color", "\\textcolor");
    try {
      var e = this.parseExpression(!1);
      return this.expect("EOF"), this.settings.globalGroup || this.gullet.endGroup(), e;
    } finally {
      this.gullet.endGroups();
    }
  }
  /**
   * Fully parse a separate sequence of tokens as a separate job.
   * Tokens should be specified in reverse order, as in a MacroDefinition.
   */
  subparse(e) {
    var t = this.nextToken;
    this.consume(), this.gullet.pushToken(new T0("}")), this.gullet.pushTokens(e);
    var a = this.parseExpression(!1);
    return this.expect("}"), this.nextToken = t, a;
  }
  /**
   * Parses an "expression", which is a list of atoms.
   *
   * `breakOnInfix`: Should the parsing stop when we hit infix nodes? This
   *                 happens when functions have higher precedence than infix
   *                 nodes in implicit parses.
   *
   * `breakOnTokenText`: The text of the token that the expression should end
   *                     with, or `null` if something else should end the
   *                     expression.
   */
  parseExpression(e, t) {
    for (var a = []; ; ) {
      this.mode === "math" && this.consumeSpaces();
      var n = this.fetch();
      if (rt.endOfExpression.has(n.text) || t && n.text === t || e && J0[n.text] && J0[n.text].infix)
        break;
      var s = this.parseAtom(t);
      if (s) {
        if (s.type === "internal")
          continue;
      } else break;
      a.push(s);
    }
    return this.mode === "text" && this.formLigatures(a), this.handleInfixNodes(a);
  }
  /**
   * Rewrites infix operators such as \over with corresponding commands such
   * as \frac.
   *
   * There can only be one infix operator per group.  If there's more than one
   * then the expression is ambiguous.  This can be resolved by adding {}.
   */
  handleInfixNodes(e) {
    for (var t = -1, a, n = 0; n < e.length; n++) {
      var s = e[n];
      if (s.type === "infix") {
        if (t !== -1)
          throw new E("only one infix operator per group", s.token);
        t = n, a = s.replaceWith;
      }
    }
    if (t !== -1 && a) {
      var u, o, h = e.slice(0, t), m = e.slice(t + 1);
      h.length === 1 && h[0].type === "ordgroup" ? u = h[0] : u = {
        type: "ordgroup",
        mode: this.mode,
        body: h
      }, m.length === 1 && m[0].type === "ordgroup" ? o = m[0] : o = {
        type: "ordgroup",
        mode: this.mode,
        body: m
      };
      var v;
      return a === "\\\\abovefrac" ? v = this.callFunction(a, [u, e[t], o], []) : v = this.callFunction(a, [u, o], []), [v];
    } else
      return e;
  }
  /**
   * Handle a subscript or superscript with nice errors.
   */
  handleSupSubscript(e) {
    var t = this.fetch(), a = t.text;
    this.consume(), this.consumeSpaces();
    var n;
    do {
      var s;
      n = this.parseGroup(e);
    } while (((s = n) == null ? void 0 : s.type) === "internal");
    if (!n)
      throw new E("Expected group after '" + a + "'", t);
    return n;
  }
  /**
   * Converts the textual input of an unsupported command into a text node
   * contained within a color node whose color is determined by errorColor
   */
  formatUnsupportedCmd(e) {
    for (var t = [], a = 0; a < e.length; a++)
      t.push({
        type: "textord",
        mode: "text",
        text: e[a]
      });
    var n = {
      type: "text",
      mode: this.mode,
      body: t
    }, s = {
      type: "color",
      mode: this.mode,
      color: this.settings.errorColor,
      body: [n]
    };
    return s;
  }
  /**
   * Parses a group with optional super/subscripts.
   */
  parseAtom(e) {
    var t = this.parseGroup("atom", e);
    if ((t == null ? void 0 : t.type) === "internal" || this.mode === "text")
      return t;
    for (var a, n; ; ) {
      this.consumeSpaces();
      var s = this.fetch();
      if (s.text === "\\limits" || s.text === "\\nolimits") {
        if (t && t.type === "op")
          t.limits = s.text === "\\limits", t.alwaysHandleSupSub = !0;
        else if (t && t.type === "operatorname")
          t.alwaysHandleSupSub && (t.limits = s.text === "\\limits");
        else
          throw new E("Limit controls must follow a math operator", s);
        this.consume();
      } else if (s.text === "^") {
        if (a)
          throw new E("Double superscript", s);
        a = this.handleSupSubscript("superscript");
      } else if (s.text === "_") {
        if (n)
          throw new E("Double subscript", s);
        n = this.handleSupSubscript("subscript");
      } else if (s.text === "'") {
        if (a)
          throw new E("Double superscript", s);
        var u = {
          type: "textord",
          mode: this.mode,
          text: "\\prime"
        }, o = [u];
        for (this.consume(); this.fetch().text === "'"; )
          o.push(u), this.consume();
        this.fetch().text === "^" && o.push(this.handleSupSubscript("superscript")), a = {
          type: "ordgroup",
          mode: this.mode,
          body: o
        };
      } else if (He[s.text]) {
        var h = jr.test(s.text), m = [];
        for (m.push(new T0(He[s.text])), this.consume(); ; ) {
          var v = this.fetch().text;
          if (!He[v] || jr.test(v) !== h)
            break;
          m.unshift(new T0(He[v])), this.consume();
        }
        var b = this.subparse(m);
        h ? n = {
          type: "ordgroup",
          mode: "math",
          body: b
        } : a = {
          type: "ordgroup",
          mode: "math",
          body: b
        };
      } else
        break;
    }
    return a && n ? {
      type: "supsub",
      mode: this.mode,
      base: t,
      sup: a,
      sub: n
    } : a ? {
      type: "supsub",
      mode: this.mode,
      base: t,
      sup: a
    } : n ? {
      type: "supsub",
      mode: this.mode,
      base: t,
      sub: n
    } : t;
  }
  /**
   * Parses an entire function, including its base and all of its arguments.
   */
  parseFunction(e, t) {
    var a = this.fetch(), n = a.text, s = J0[n];
    if (!s)
      return null;
    if (this.consume(), t && t !== "atom" && !s.allowedInArgument)
      throw new E("Got function '" + n + "' with no arguments" + (t ? " as " + t : ""), a);
    if (this.mode === "text" && !s.allowedInText)
      throw new E("Can't use function '" + n + "' in text mode", a);
    if (this.mode === "math" && s.allowedInMath === !1)
      throw new E("Can't use function '" + n + "' in math mode", a);
    var {
      args: u,
      optArgs: o
    } = this.parseArguments(n, s);
    return this.callFunction(n, u, o, a, e);
  }
  /**
   * Call a function handler with a suitable context and arguments.
   */
  callFunction(e, t, a, n, s) {
    var u = {
      funcName: e,
      parser: this,
      token: n,
      breakOnTokenText: s
    }, o = J0[e];
    if (o && o.handler)
      return o.handler(u, t, a);
    throw new E("No function handler for " + e);
  }
  /**
   * Parses the arguments of a function or environment
   */
  parseArguments(e, t) {
    var a, n = (a = t.numOptionalArgs) != null ? a : 0, s = t.numArgs + n;
    if (s === 0)
      return {
        args: [],
        optArgs: []
      };
    for (var u = [], o = [], h = 0; h < s; h++) {
      var m, v = (m = t.argTypes) == null ? void 0 : m[h], b = h < n;
      ("primitive" in t && t.primitive && v == null || // \sqrt expands into primitive if optional argument doesn't exist
      t.type === "sqrt" && h === 1 && o[0] == null) && (v = "primitive");
      var k = this.parseGroupOfType("argument to '" + e + "'", v, b);
      if (b)
        o.push(k);
      else if (k != null)
        u.push(k);
      else
        throw new E("Null argument, please report this as a bug");
    }
    return {
      args: u,
      optArgs: o
    };
  }
  /**
   * Parses a group when the mode is changing.
   */
  parseGroupOfType(e, t, a) {
    switch (t) {
      case "color":
        return this.parseColorGroup(a);
      case "size":
        return this.parseSizeGroup(a);
      case "url":
        return this.parseUrlGroup(a);
      case "math":
      case "text":
        return this.parseArgumentGroup(a, t);
      case "hbox": {
        var n = this.parseArgumentGroup(a, "text");
        return n != null ? {
          type: "styling",
          mode: n.mode,
          body: [n],
          style: "text",
          // simulate \textstyle
          resetFont: !0
        } : null;
      }
      case "raw": {
        var s = this.parseStringGroup(a);
        return s != null ? {
          type: "raw",
          mode: "text",
          string: s.text
        } : null;
      }
      case "primitive": {
        if (a)
          throw new E("A primitive argument cannot be optional");
        var u = this.parseGroup(e);
        if (u == null)
          throw new E("Expected group as " + e, this.fetch());
        return u;
      }
      case "original":
      case void 0:
        return this.parseArgumentGroup(a);
      default:
        throw new E("Unknown group type as " + e, this.fetch());
    }
  }
  /**
   * Discard any space tokens, fetching the next non-space token.
   */
  consumeSpaces() {
    for (; this.fetch().text === " "; )
      this.consume();
  }
  /**
   * Parses a group, essentially returning the string formed by the
   * brace-enclosed tokens plus some position information.
   */
  parseStringGroup(e) {
    var t = this.gullet.scanArgument(e);
    if (t == null)
      return null;
    for (var a = "", n; (n = this.fetch()).text !== "EOF"; )
      a += n.text, this.consume();
    return this.consume(), t.text = a, t;
  }
  /**
   * Parses a regex-delimited group: the largest sequence of tokens
   * whose concatenated strings match `regex`. Returns the string
   * formed by the tokens plus some position information.
   */
  parseRegexGroup(e, t) {
    for (var a = this.fetch(), n = a, s = "", u; (u = this.fetch()).text !== "EOF" && e.test(s + u.text); )
      n = u, s += n.text, this.consume();
    if (s === "")
      throw new E("Invalid " + t + ": '" + a.text + "'", a);
    return a.range(n, s);
  }
  /**
   * Parses a color description.
   */
  parseColorGroup(e) {
    var t = this.parseStringGroup(e);
    if (t == null)
      return null;
    var a = /^(#[a-f0-9]{3,4}|#[a-f0-9]{6}|#[a-f0-9]{8}|[a-f0-9]{6}|[a-z]+)$/i.exec(t.text);
    if (!a)
      throw new E("Invalid color: '" + t.text + "'", t);
    var n = a[0];
    return /^[0-9a-f]{6}$/i.test(n) && (n = "#" + n), {
      type: "color-token",
      mode: this.mode,
      color: n
    };
  }
  /**
   * Parses a size specification, consisting of magnitude and unit.
   */
  parseSizeGroup(e) {
    var t, a = !1;
    if (this.gullet.consumeSpaces(), !e && this.gullet.future().text !== "{" ? t = this.parseRegexGroup(/^[-+]? *(?:$|\d+|\d+\.\d*|\.\d*) *[a-z]{0,2} *$/, "size") : t = this.parseStringGroup(e), !t)
      return null;
    !e && t.text.length === 0 && (t.text = "0pt", a = !0);
    var n = /([-+]?) *(\d+(?:\.\d*)?|\.\d+) *([a-z]{2})/.exec(t.text);
    if (!n)
      throw new E("Invalid size: '" + t.text + "'", t);
    var s = {
      number: +(n[1] + n[2]),
      // sign + magnitude, cast to number
      unit: n[3]
    };
    if (!Kr(s))
      throw new E("Invalid unit: '" + s.unit + "'", t);
    return {
      type: "size",
      mode: this.mode,
      value: s,
      isBlank: a
    };
  }
  /**
   * Parses an URL, checking escaped letters and allowed protocols,
   * and setting the catcode of % as an active character (as in \hyperref).
   */
  parseUrlGroup(e) {
    this.gullet.lexer.setCatcode("%", 13), this.gullet.lexer.setCatcode("~", 12);
    var t = this.parseStringGroup(e);
    if (this.gullet.lexer.setCatcode("%", 14), this.gullet.lexer.setCatcode("~", 13), t == null)
      return null;
    var a = t.text.replace(/\\([#$%&~_^{}])/g, "$1");
    return {
      type: "url",
      mode: this.mode,
      url: a
    };
  }
  /**
   * Parses an argument with the mode specified.
   */
  parseArgumentGroup(e, t) {
    var a = this.gullet.scanArgument(e);
    if (a == null)
      return null;
    var n = this.mode;
    t && this.switchMode(t), this.gullet.beginGroup();
    var s = this.parseExpression(!1, "EOF");
    this.expect("EOF"), this.gullet.endGroup();
    var u = {
      type: "ordgroup",
      mode: this.mode,
      loc: a.loc,
      body: s
    };
    return t && this.switchMode(n), u;
  }
  /**
   * Parses an ordinary group, which is either a single nucleus (like "x")
   * or an expression in braces (like "{x+y}") or an implicit group, a group
   * that starts at the current position, and ends right before a higher explicit
   * group ends, or at EOF.
   */
  parseGroup(e, t) {
    var a = this.fetch(), n = a.text, s;
    if (n === "{" || n === "\\begingroup") {
      this.consume();
      var u = n === "{" ? "}" : "\\endgroup";
      this.gullet.beginGroup();
      var o = this.parseExpression(!1, u), h = this.fetch();
      this.expect(u), this.gullet.endGroup(), s = {
        type: "ordgroup",
        mode: this.mode,
        loc: y0.range(a, h),
        body: o,
        // A group formed by \begingroup...\endgroup is a semi-simple group
        // which doesn't affect spacing in math mode, i.e., is transparent.
        // https://tex.stackexchange.com/questions/1930/when-should-one-
        // use-begingroup-instead-of-bgroup
        semisimple: n === "\\begingroup" || void 0
      };
    } else if (s = this.parseFunction(t, e) || this.parseSymbol(), s == null && n[0] === "\\" && !Oa.hasOwnProperty(n)) {
      if (this.settings.throwOnError)
        throw new E("Undefined control sequence: " + n, a);
      s = this.formatUnsupportedCmd(n), this.consume();
    }
    return s;
  }
  /**
   * Form ligature-like combinations of characters for text mode.
   * This includes inputs like "--", "---", "``" and "''".
   * The result will simply replace multiple textord nodes with a single
   * character in each value by a single textord node having multiple
   * characters in its value.  The representation is still ASCII source.
   * The group will be modified in place.
   */
  formLigatures(e) {
    for (var t = e.length - 1, a = 0; a < t; ++a) {
      var n = e[a];
      if (n.type === "textord") {
        var s = n.text, u = e[a + 1];
        if (!(!u || u.type !== "textord")) {
          if (s === "-" && u.text === "-") {
            var o = e[a + 2];
            a + 1 < t && o && o.type === "textord" && o.text === "-" ? (e.splice(a, 3, {
              type: "textord",
              mode: "text",
              loc: y0.range(n, o),
              text: "---"
            }), t -= 2) : (e.splice(a, 2, {
              type: "textord",
              mode: "text",
              loc: y0.range(n, u),
              text: "--"
            }), t -= 1);
          }
          (s === "'" || s === "`") && u.text === s && (e.splice(a, 2, {
            type: "textord",
            mode: "text",
            loc: y0.range(n, u),
            text: s + s
          }), t -= 1);
        }
      }
    }
  }
  /**
   * Parse a single symbol out of the string. Here, we handle single character
   * symbols and special functions like \verb.
   */
  parseSymbol() {
    var e = this.fetch(), t = e.text;
    if (/^\\verb[^a-zA-Z]/.test(t)) {
      this.consume();
      var a = t.slice(5), n = a.charAt(0) === "*";
      if (n && (a = a.slice(1)), a.length < 2 || a.charAt(0) !== a.slice(-1))
        throw new E(`\\verb assertion failed --
                    please report what input caused this bug`);
      return a = a.slice(1, -1), {
        type: "verb",
        mode: "text",
        body: a,
        star: n
      };
    }
    Xr.hasOwnProperty(t[0]) && !Q[this.mode][t[0]] && (this.settings.strict && this.mode === "math" && this.settings.reportNonstrict("unicodeTextInMathMode", 'Accented Unicode text character "' + t[0] + '" used in math mode', e), t = Xr[t[0]] + t.slice(1));
    var s = ds.exec(t);
    s && (t = t.substring(0, s.index), t === "i" ? t = "ı" : t === "j" && (t = "ȷ"));
    var u;
    if (Q[this.mode][t]) {
      this.settings.strict && this.mode === "math" && Mt.includes(t) && this.settings.reportNonstrict("unicodeTextInMathMode", 'Latin-1/Unicode text character "' + t[0] + '" used in math mode', e);
      var o = Q[this.mode][t].group, h = y0.range(e), m;
      ki(o) ? m = {
        type: "atom",
        mode: this.mode,
        family: o,
        loc: h,
        text: t
      } : m = {
        type: o,
        mode: this.mode,
        loc: h,
        text: t
      }, u = m;
    } else if (t.charCodeAt(0) >= 128)
      this.settings.strict && (Zr(t.charCodeAt(0)) ? this.mode === "math" && this.settings.reportNonstrict("unicodeTextInMathMode", 'Unicode text character "' + t[0] + '" used in math mode', e) : this.settings.reportNonstrict("unknownSymbol", 'Unrecognized Unicode character "' + t[0] + '"' + (" (" + t.charCodeAt(0) + ")"), e)), u = {
        type: "textord",
        mode: "text",
        loc: y0.range(e),
        text: t
      };
    else
      return null;
    if (this.consume(), s)
      for (var v = 0; v < s[0].length; v++) {
        var b = s[0][v];
        if (!xt[b])
          throw new E("Unknown accent ' " + b + "'", e);
        var k = xt[b][this.mode] || xt[b].text;
        if (!k)
          throw new E("Accent " + b + " unsupported in " + this.mode + " mode", e);
        u = {
          type: "accent",
          mode: this.mode,
          loc: y0.range(e),
          label: k,
          isStretchy: !1,
          isShifty: !0,
          base: u
        };
      }
    return u;
  }
}
rt.endOfExpression = /* @__PURE__ */ new Set(["}", "\\endgroup", "\\end", "\\right", "&"]);
var rr = function(e, t) {
  if (!(typeof e == "string" || e instanceof String))
    throw new TypeError("KaTeX can only parse string typed expression");
  var a = new rt(e, t);
  delete a.gullet.macros.current["\\df@tag"];
  var n = a.parse();
  if (delete a.gullet.macros.current["\\current@color"], delete a.gullet.macros.current["\\color"], a.gullet.macros.get("\\df@tag")) {
    if (!t.displayMode)
      throw new E("\\tag works only in display equations");
    n = [{
      type: "tag",
      mode: "text",
      body: n,
      tag: a.subparse([new T0("\\df@tag")])
    }];
  }
  return n;
}, La = function(e, t, a) {
  t.textContent = "";
  var n = ar(e, a).toNode();
  t.appendChild(n);
};
typeof document < "u" && document.compatMode !== "CSS1Compat" && (typeof console < "u" && console.warn("Warning: KaTeX doesn't work in quirks mode. Make sure your website has a suitable doctype."), La = function() {
  throw new E("KaTeX doesn't work in quirks mode.");
});
var bs = function(e, t) {
  var a = ar(e, t).toMarkup();
  return a;
}, ys = function(e, t) {
  var a = new Gt(t);
  return rr(e, a);
}, Ha = function(e, t, a) {
  if (a.throwOnError || !(e instanceof E))
    throw e;
  var n = F(["katex-error"], [new z0(t)]);
  return n.setAttribute("title", e.toString()), n.setAttribute("style", "color:" + a.errorColor), n;
}, ar = function(e, t) {
  var a = new Gt(t);
  try {
    var n = rr(e, a);
    return pi(n, e, a);
  } catch (s) {
    return Ha(s, e, a);
  }
}, xs = function(e, t) {
  var a = new Gt(t);
  try {
    var n = rr(e, a);
    return vi(n, e, a);
  } catch (s) {
    return Ha(s, e, a);
  }
}, ws = "0.17.0", Ss = {
  Span: be,
  Anchor: Xe,
  SymbolNode: z0,
  SvgNode: G0,
  PathNode: ee,
  LineNode: zt
}, ks = {
  /**
   * Current KaTeX version
   */
  version: ws,
  /**
   * Renders the given LaTeX into an HTML+MathML combination, and adds
   * it as a child to the specified DOM node.
   */
  render: La,
  /**
   * Renders the given LaTeX into an HTML+MathML combination string,
   * for sending to the client.
   */
  renderToString: bs,
  /**
   * KaTeX error, usually during parsing.
   */
  ParseError: E,
  /**
   * The schema of Settings
   */
  SETTINGS_SCHEMA: kt,
  /**
   * Parses the given LaTeX into KaTeX's internal parse tree structure,
   * without rendering to HTML or MathML.
   *
   * NOTE: This method is not currently recommended for public use.
   * The internal tree representation is unstable and is very likely
   * to change. Use at your own risk.
   */
  __parse: ys,
  /**
   * Renders the given LaTeX into an HTML+MathML internal DOM tree
   * representation, without flattening that representation to a string.
   *
   * NOTE: This method is not currently recommended for public use.
   * The internal tree representation is unstable and is very likely
   * to change. Use at your own risk.
   */
  __renderToDomTree: ar,
  /**
   * Renders the given LaTeX into an HTML internal DOM tree representation,
   * without MathML and without flattening that representation to a string.
   *
   * NOTE: This method is not currently recommended for public use.
   * The internal tree representation is unstable and is very likely
   * to change. Use at your own risk.
   */
  __renderToHTMLTree: xs,
  /**
   * extends internal font metrics object with a new object
   * each key in the new object represents a font name
  */
  __setFontMetrics: Zn,
  /**
   * adds a new symbol to builtin symbols table
   */
  __defineSymbol: i,
  /**
   * adds a new function to builtin function list,
   * which directly produce parse tree elements
   * and have their own html/mathml builders
   */
  __defineFunction: O,
  /**
   * adds a new macro to builtin macro list
   */
  __defineMacro: d,
  /**
   * Expose the dom tree node types, which can be useful for type checking nodes.
   *
   * NOTE: These methods are not currently recommended for public use.
   * The internal tree representation is unstable and is very likely
   * to change. Use at your own risk.
   */
  __domTree: Ss
};
function As(r, e) {
  return ks.renderToString(r, {
    displayMode: e,
    output: "htmlAndMathml",
    throwOnError: !1,
    strict: "ignore",
    errorColor: "#a6300e",
    trust: !1
  });
}
function Ks({ tex: r, block: e = !1, caption: t }) {
  const a = r != null && r !== "", n = en(
    () => a ? As(r, e) : "",
    [e, a, r]
  ), s = a ? /* @__PURE__ */ y(
    "span",
    {
      className: "ra-formula__math",
      dangerouslySetInnerHTML: { __html: n }
    }
  ) : /* @__PURE__ */ y(J, { label: "公式 tex" });
  return e ? /* @__PURE__ */ $("figure", { className: "ra-formula ra-formula--block", children: [
    /* @__PURE__ */ y("div", { className: "ra-formula__body", children: s }),
    t ? /* @__PURE__ */ y("figcaption", { className: "ra-formula__caption", children: t }) : null
  ] }) : /* @__PURE__ */ y("span", { className: "ra-formula ra-formula--inline", children: s });
}
const Ts = { add: "+", del: "-", ctx: " " };
function Js({ file: r, title: e, lines: t, notes: a }) {
  return /* @__PURE__ */ $("div", { className: "ra-diff", children: [
    /* @__PURE__ */ $("div", { className: "ra-diff__head", children: [
      /* @__PURE__ */ y("span", { className: "ra-diff__label", children: e ?? "变更审阅" }),
      r ? /* @__PURE__ */ y("span", { className: "ra-diff__file", children: r }) : null
    ] }),
    t && t.length > 0 ? /* @__PURE__ */ y("pre", { className: "ra-diff__code", children: t.map((n, s) => {
      const u = n.type ?? "ctx";
      return /* @__PURE__ */ $("span", { className: `ra-diff__line ra-diff__line--${u}`, children: [
        /* @__PURE__ */ y("span", { className: "ra-diff__gutter", children: Ts[u] }),
        /* @__PURE__ */ y("span", { className: "ra-diff__text", children: n.text ?? "" }),
        `
`
      ] }, s);
    }) }) : /* @__PURE__ */ y("div", { className: "ra-diff__missing", children: /* @__PURE__ */ y(J, { label: "变更内容" }) }),
    a && a.length > 0 ? /* @__PURE__ */ y("ul", { className: "ra-diff__notes", children: a.map((n, s) => /* @__PURE__ */ $("li", { className: "ra-diff__note", children: [
      n.ref ? /* @__PURE__ */ y("span", { className: "ra-diff__ref", children: n.ref }) : null,
      /* @__PURE__ */ y("span", { className: "ra-diff__note-text", children: c0(n.text, "审阅意见") })
    ] }, s)) }) : null
  ] });
}
function Qs({ summary: r, children: e, open: t }) {
  return /* @__PURE__ */ $("details", { className: "ra-detail", open: t, children: [
    /* @__PURE__ */ y("summary", { className: "ra-detail__summary", children: c0(r, "摘要行") }),
    /* @__PURE__ */ y("div", { className: "ra-detail__body", children: e })
  ] });
}
function e1({ tabs: r, initial: e = 0 }) {
  const t = Pt(), [a, n] = ze(e);
  if (r == null || r.length === 0)
    return /* @__PURE__ */ y(J, { label: "标签页" });
  const s = Math.min(Math.max(a, 0), r.length - 1), u = r[s];
  return /* @__PURE__ */ $("div", { className: "ra-tabs", children: [
    /* @__PURE__ */ y("div", { className: "ra-tabs__list", role: "tablist", children: r.map((o, h) => /* @__PURE__ */ y(
      "button",
      {
        type: "button",
        role: "tab",
        id: `${t}-tab-${h}`,
        "aria-selected": h === s,
        className: "ra-tabs__tab" + (h === s ? " ra-tabs__tab--active" : ""),
        onClick: () => n(h),
        children: o.label
      },
      h
    )) }),
    /* @__PURE__ */ y("div", { className: "ra-tabs__panel", role: "tabpanel", children: u.content })
  ] });
}
function t1({ title: r, html: e, children: t }) {
  return /* @__PURE__ */ $("div", { className: "ra-raw", children: [
    e != null ? /* @__PURE__ */ y(
      "div",
      {
        className: "ra-raw__body",
        dangerouslySetInnerHTML: { __html: e }
      }
    ) : /* @__PURE__ */ y("div", { className: "ra-raw__body", children: t }),
    r ? /* @__PURE__ */ y("div", { className: "ra-raw__caption", children: r }) : null
  ] });
}
function r1({ actionItems: r, decision: e }) {
  const [t, a] = ze(""), n = (u) => {
    a(u), window.setTimeout(() => a(""), 1500);
  }, s = async (u, o) => {
    const h = await Yr(u);
    n(h ? `${o}已复制` : "复制失败");
  };
  return /* @__PURE__ */ $("div", { className: "ra-export-bar no-print", children: [
    /* @__PURE__ */ y("span", { className: "ra-export-bar__title", children: "导出" }),
    /* @__PURE__ */ y("button", { onClick: () => window.print(), children: "导出 PDF（打印）" }),
    r ? /* @__PURE__ */ y(
      "button",
      {
        onClick: () => s(un(r), "Action Items"),
        children: "复制为 Action Items"
      }
    ) : null,
    e ? /* @__PURE__ */ y("button", { onClick: () => s(on(e), "Prompt"), children: "复制为 Prompt" }) : null,
    t ? /* @__PURE__ */ y("span", { className: "ra-export-bar__title", children: t }) : null
  ] });
}
export {
  Xs as ActionList,
  Es as Article,
  Os as Aside,
  Gs as Audio,
  Vs as Checkpoint,
  Zs as CodeBlock,
  qs as Conclusion,
  js as Decision,
  Qs as Detail,
  Js as DiffReview,
  r1 as ExportBar,
  Ks as Formula,
  Ds as Hero,
  pn as HighlightedCode,
  $s as Image,
  Ws as Incident,
  Bs as Lead,
  J as MissingField,
  Ls as Quote,
  t1 as Raw,
  Us as RiskList,
  Is as Section,
  Rs as Subsection,
  _s as Summary,
  Fs as THEMES,
  Ns as THEME_LABELS,
  tn as TOC,
  Hs as Table,
  e1 as Tabs,
  Cs as ThemeProvider,
  Ys as Tradeoff,
  Ps as Video,
  un as actionItemsToMarkdown,
  Yr as copyToClipboard,
  on as decisionToPrompt
};
