#!/usr/bin/env python3
"""Regenerate docs/testA-before-after.png from the measured testA output.

Run from the repository root:  py -X utf8 tools/make-testa-shot.py
Requires Pillow (py -X utf8 -c "import PIL") and the Windows fonts below.
The image embeds the *actual* numbers asserted by the golden suite:
  testA-no-profile -> total 4 / behavioral 2 / cosmetic 2 / exit 1
  testA-profile    -> total 2 / behavioral 0 / cosmetic 2 / exit 0
Changing the CLI output format means regenerating this file and the picture.
"""
from PIL import Image, ImageDraw, ImageFont

W, H = 1280, 760
BG = (17, 22, 33)
PANEL = (24, 31, 46)
FG = (226, 232, 240)
MUTED = (148, 163, 184)
RED = (248, 113, 113)
GREEN = (74, 222, 128)
BLUE = (96, 165, 250)

img = Image.new("RGB", (W, H), BG)
d = ImageDraw.Draw(img)

def font(cjk, size, bold=False):
    path = r"C:\Windows\Fonts\msyhbd.ttc" if (cjk and bold) else (
        r"C:\Windows\Fonts\msyh.ttc" if cjk else (
        r"C:\Windows\Fonts\consolab.ttf" if bold else r"C:\Windows\Fonts\consola.ttf"))
    return ImageFont.truetype(path, size)

def put(x, y, text, fg=FG, size=20, bold=False):
    cjk = any(ord(ch) > 127 for ch in text)
    d.text((x, y), text, font=font(cjk, size, bold), fill=fg)

put(48, 36, "cfgdelta · 测试A：默认值双侧抵消", FG, 34, True)
put(48, 86, "testdata/golden/testA/k1.yaml  vs  k2.yaml —— 4 处书写差异，其中 2 处是「一侧缺失、另一侧显式写默认值」", MUTED, 18)

# left panel
d.rounded_rectangle([48, 136, 620, 610], radius=14, fill=PANEL, outline=(55, 65, 81), width=2)
put(72, 156, "BEFORE  ·  no profile table", RED, 20, True)
put(72, 190, "cfgdelta diff k1.yaml k2.yaml", MUTED, 16)
left = [
    ("- /…/0/imagePullPolicy", "string:IfNotPresent", "[配置·新增·移除 (class_guess)]"),
    ("- /…/0/ports/0/protocol", "string:TCP", "[配置·新增·移除 (class_guess)]"),
]
y = 236
for p, v, c in left:
    put(72, y, p, FG, 17)
    put(72, y+24, "  " + v + "  " + c, MUTED, 16)
    y += 62
put(72, 392, "summary: total 4 · behavioral 2 · cosmetic 2 · indeterminate 0", FG, 16)
put(72, 460, "exit 1", RED, 30, True)
put(72, 510, "两条「无中生有」的默认值被当成行为差异；", MUTED, 16)
put(72, 534, "真正的两处单位写法只计为 cosmetic。", MUTED, 16)

# right panel
d.rounded_rectangle([660, 136, 1232, 610], radius=14, fill=PANEL, outline=(55, 65, 81), width=2)
put(684, 156, "AFTER  ·  --profile k8s/v1.29", GREEN, 20, True)
put(684, 190, "cfgdelta diff --profile k8s/v1.29 k1.yaml k2.yaml", MUTED, 16)
put(684, 240, "profile k8s/v1.29 (hits: 14)", FG, 17)
put(684, 264, "defaults applied on BOTH sides → cancel", MUTED, 16)
put(684, 340, "equivalent (no behavioural differences)", GREEN, 19, True)
put(684, 392, "summary: total 2 · behavioral 0 · cosmetic 2 · indeterminate 0", FG, 16)
put(684, 460, "exit 0", GREEN, 30, True)
put(684, 510, "显式非默认值仍会报（testA-ctrl-profile → exit 1）；", MUTED, 16)
put(684, 534, "未命中任何规则时头部标 degraded: structural-only。", MUTED, 16)

put(48, 650, "golden: testA-no-profile (exit 1)  →  testA-profile (exit 0)   ·   cfgdelta selftest 断言退出码 + summary 计数", MUTED, 17)
put(48, 686, "本图为 M4 实测输出；不是示意图。复跑命令见 README「Quick start」。", MUTED, 17)

img.save("docs/testA-before-after.png", "PNG", optimize=True)
print("wrote docs/testA-before-after.png", img.size)
