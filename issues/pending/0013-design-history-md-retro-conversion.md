---
decision_type: process
affected_paths:
  - docs/history.md
tags: [intent, history, retro, decision-record]
---

# docs/history.md の既存 11 件を issue 化するか残置するかの方針決定

Created: 2026-05-10

## 概要

0011 で意思決定の primary store を `issues/` に集約し、`make intent P=<p>` で
コードから逆引きできるようにした。`docs/history.md` には 0011 以前から積まれた
過去の意思決定 11 件があり、これらは現状 issue 化されていないため、
`make intent` の対象外。retro 的に issue として起こすか、history.md に
ナラティブとして残置するか、ハイブリッドにするかを未決のまま放置すると、
新規決定は issue / 過去決定は history.md という分断が固定化する。

## 根拠

- 0011 後の運用では「issue が primary、history.md は事後ナラティブ要約」と
  CLAUDE.md で明文化したが、既存 11 件はその新ルールの前に書かれている
- `make intent` は `affected_paths` を見るので、history.md にしかない決定は
  Claude / Codex から逆引きできない（最も agentic に重要な過去決定が
  検索網に乗らない）
- 一方、history.md は「人間が大方針を筋立てたナラティブ」として書かれており、
  機械可読な issue 形式に全 11 件を分解すると冗長 / コンテキスト喪失のリスク

## 問題

- どの 11 件が issue 化に値するか（agentic に逆引きされる蓋然性で選別すべきか、
  全件機械的に起こすべきか、選別基準は何か）
- issue 化する場合、`affected_paths` が現存する path とずれる可能性
  （rename / 削除済み）。`--lint` で warning にはなるが、retro 化時に
  人手で path を解決する必要がある
- issue 化しない場合、history.md エントリにも `make intent` 相当の
  逆引きを効かせる仕組みが要るか（例: history.md 内の各節に
  affected_paths メタを付ける拡張）
- 既に書かれた history.md の節を issue にコピーする労力 vs 得られる
  逆引きヒット率の見積り

## 対応方針

未確定。pending として保留する。決定すべき軸:

1. **選別基準**: 全件起こす / `affected_paths` を引きやすそうな実装系の決定だけ起こす / 起こさず history.md 拡張で対応
2. **保留候補**: history.md L1-L11（番号付きの 11 件）を一覧化し、それぞれに
   「この決定は将来 `make intent` で引かれそうか」を Yes/No 評価する
3. **ハイブリッド時の重複ルール**: 同じ決定が history.md と issue 双方に
   存在する場合、どちらを正とするか（issue が正、history.md は要約のみ
   残す等）

着手前に、まず 0012 (Hugo docs site) の段階で history.md がどう扱われるか
（site 上の表示形態）を見極めると判断材料が増える。

## Pending 2026-05-10

0011 を実装した直後の判断: retro 化の労力対効果は実運用してみないと分から
ない。`make intent` で過去決定が引けないことが実害として顕在化したタイミング
（例: ある PR で過去判断を見落として手戻りが発生した、等）で着手するのが
順当。0012 の Hugo build 方針が固まれば、history.md の表示形態と issue の
表示形態を合わせる必要が出るので、その時点で再検討する。
