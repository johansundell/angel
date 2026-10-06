# Logo Design Concepts for Angel

This document outlines brand identity and logo concept proposals for **Angel** (Daily Messaging and Caregiver Communication System for Swedish home care / *hemtjänst*).

---

## Brand Personality & Requirements

- **Role & Audience**: Used daily by visiting caregivers (*hemtjänstpersonal*) on smartphones and by the care recipient (*kunden*) at home.
- **Values**: Warmth, dependability, care, calm, clarity, and safety.
- **Aesthetic**: Nordic / Scandinavian minimalism, high legibility, clean geometry, friendly rather than overly corporate or medical.
- **Technical Needs**:
  - Must scale cleanly from a 32×32 favicon / app icon to an on-screen header.
  - Seamless support for both light and dark display modes (`--bg: #f8fafc` / `#0f172a`).
  - Available as scalable vector graphics (`.svg`) and high-resolution raster formats (`.png`).

---

## The Concepts

### Concept 1: The Embracing Wings (*De skyddande vingarna*)
- **Visual Motif**: A golden warm arch / sun silhouette flanked by stylized symmetrical blue wings forming an embrace.
- **Symbolism**: Represents protection, safe arrival, and warmth. The wings evoke both the name "Angel" and the caring hands of visiting staff.
- **Palette**: Warm Amber (`#F59E0B`), Cobalt / Royal Blue (`#2563EB`, `--primary`), Deep Slate (`#0F172A`).
- **Files**:
  - Vector: [`assets/img/concepts/concept-1-wings.svg`](file:///home/johan/repos/johansundell/angel/assets/img/concepts/concept-1-wings.svg)
  - Render Preview: `assets/img/concepts/concept-1-wings.jpg`

---

### Concept 2: Heart & Wings Line Art (*Hjärta & Vingar*)
- **Visual Motif**: Continuous fluid line art blending a welcoming heart with wings, accented in gentle coral and teal.
- **Symbolism**: Centers on personal empathy, compassion, and human connection in home healthcare.
- **Palette**: Soft Teal (`#0D9488`), Rose/Coral (`#F43F5E`), Muted Slate (`#64748B`).
- **Files**:
  - Vector: [`assets/img/concepts/concept-2-heart.svg`](file:///home/johan/repos/johansundell/angel/assets/img/concepts/concept-2-heart.svg)
  - Render Preview: `assets/img/concepts/concept-2-heart.jpg`

---

### Concept 3: The Guiding Halo Note (*Anteckning & Gloria*)
- **Visual Motif**: A minimalist message / daily note card with a warm, gentle halo hovering above it.
- **Symbolism**: Directly symbolizes the core function of the application: communication, notes left for visitors, and reassurance.
- **Palette**: Golden Halo (`#F59E0B`), Deep Night Sky (`#1E293B`), Sky Blue Accent (`#38BDF8`).
- **Files**:
  - Vector: [`assets/img/concepts/concept-3-halo-note.svg`](file:///home/johan/repos/johansundell/angel/assets/img/concepts/concept-3-halo-note.svg)
  - Render Preview: `assets/img/concepts/concept-3-halo-note.jpg`

---

---

## Status & Integration

**Selected Concept**: **Concept 3: The Guiding Halo Note (*Gloria & Anteckning*)** was selected as the brand identity for Angel.

### Production Assets Delivered

1. **Favicon**:
   - Vector: [`assets/img/favicon.svg`](file:///home/johan/repos/johansundell/angel/assets/img/favicon.svg) (adaptive light & dark modes)
   - Raster: [`assets/img/favicon.png`](file:///home/johan/repos/johansundell/angel/assets/img/favicon.png) (96×96 RGBA crisp emblem)
   - Linked in [`tmpl/base.html`](file:///home/johan/repos/johansundell/angel/tmpl/base.html) with SVG priority and PNG fallback.
2. **App Logo**:
   - Vector: [`assets/img/logo.svg`](file:///home/johan/repos/johansundell/angel/assets/img/logo.svg) (adaptive light & dark styling, clean typography)
   - Raster: [`assets/img/logo.png`](file:///home/johan/repos/johansundell/angel/assets/img/logo.png) (replacing legacy template image)
3. **App Header**:
   - Integrated into the PIN entry screen [`tmpl/entry.html`](file:///home/johan/repos/johansundell/angel/tmpl/entry.html) with `.entry-logo` responsive styling in [`assets/css/main.css`](file:///home/johan/repos/johansundell/angel/assets/css/main.css).
