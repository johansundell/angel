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

## Integration Plan Once Selected

1. **Favicon**: Generate 16×16, 32×32, 180×180 apple-touch-icon and `assets/img/favicon.png` from the chosen vector emblem.
2. **App Header**: Embed the vector logo above the entry keypad on `/` and optional compact icon on `/note` and `/admin`.
3. **Template Cleanup**: Replace the legacy template logo at `assets/img/logo.png`.
