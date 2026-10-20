#!/usr/bin/env bash
# 重抓竞品基线留档（testdata/baseline/<tool>/）。
#
# 用法（在**仓库根**执行，三个环境变量指向你本机取到的同版本二进制）：
#   DYFF=/path/to/dyff DIFFYML=/path/to/diffyml CONFDIFF=/path/to/confdiff \
#     bash tools/capture-baseline.sh
#
# 为什么用环境变量：三家二进制不入库，而仓库纪律禁止把本机绝对路径写进库内文档。
# 本脚本写出的 COMMANDS.txt 里 argv[0] 一律是**工具名**，产物可直接提交。
#
# 纪律（data/README.md §1.2）：输出必须是**原样** stdout+stderr，不得手工修饰；
# 上游行为变了就追加带日期的复核记录，不删旧档。
set -u
ROOT="$(pwd)"
OUT="$ROOT/testdata/baseline"
DYFF="${DYFF:?set DYFF to the dyff binary}"
DIFFYML="${DIFFYML:?set DIFFYML to the diffyml binary}"
CONFDIFF="${CONFDIFF:?set CONFDIFF to the confdiff binary}"

emit() { # emit <tool> <case> <cmd...>
  local tool="$1" case_id="$2"; shift 2
  local dir="$OUT/$tool"
  mkdir -p "$dir"
  local code
  "$@" > "$dir/$case_id.out" 2>&1
  code=$?
  printf '%s=%s\n' "$case_id" "$code" >> "$dir/exit_code.txt"
  # argv[0] 写工具名，其余逐字保留。
  printf 'case=%s cwd=repo-root cmd=%s %s\n' "$case_id" "$tool" "${*:2}" >> "$dir/COMMANDS.txt"
}

for t in dyff diffyml confdiff; do
  mkdir -p "$OUT/$t"
  : > "$OUT/$t/exit_code.txt"
  : > "$OUT/$t/COMMANDS.txt"
  rm -f "$OUT/$t"/*.out
done

# ---- 版本（非空自证；输出落 VERSION.txt，不额外写 version.out）----
"$DYFF" version     > "$OUT/dyff/VERSION.txt" 2>&1
"$DIFFYML" --version > "$OUT/diffyml/VERSION.txt" 2>&1
"$CONFDIFF" --version > "$OUT/confdiff/VERSION.txt" 2>&1
printf 'case=version cwd=repo-root cmd=dyff version\n'        >> "$OUT/dyff/COMMANDS.txt"
printf 'case=version cwd=repo-root cmd=diffyml --version\n'   >> "$OUT/diffyml/COMMANDS.txt"
printf 'case=version cwd=repo-root cmd=confdiff --version\n'  >> "$OUT/confdiff/COMMANDS.txt"
printf 'version=0\n' >> "$OUT/dyff/exit_code.txt"
printf 'version=0\n' >> "$OUT/diffyml/exit_code.txt"
printf 'version=0\n' >> "$OUT/confdiff/exit_code.txt"

# ---- 测试A ----
emit dyff     A-dyff     "$DYFF" between --set-exit-code testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml
emit diffyml  A-diffyml  "$DIFFYML" testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml
emit diffyml  A-diffyml-exitcode "$DIFFYML" --set-exit-code testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml
emit confdiff A-confdiff "$CONFDIFF" testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml --no-color

# ---- 测试B ----
emit dyff     B-dyff "$DYFF" between --set-exit-code testdata/golden/testB/va.yaml testdata/golden/testB/vb.yaml
emit confdiff B-confdiff "$CONFDIFF" testdata/golden/testB/va.yaml testdata/golden/testB/vb.yaml -l --no-color
emit confdiff B-confdiff-array-set "$CONFDIFF" testdata/golden/testB/va.yaml testdata/golden/testB/vb.yaml -l --array-set --no-color

# ---- 测试C（c01..c07 应当判等；c08 反例必须报；c09 歧义）----
for i in 01 02 03 04 05 06 07 08 09; do
  emit dyff     "C$i-dyff"     "$DYFF" between --set-exit-code "testdata/golden/testC/c$i-left.yaml" "testdata/golden/testC/c$i-right.yaml"
  emit diffyml  "C$i-diffyml"  "$DIFFYML" "testdata/golden/testC/c$i-left.yaml" "testdata/golden/testC/c$i-right.yaml"
  emit confdiff "C$i-confdiff" "$CONFDIFF" "testdata/golden/testC/c$i-left.yaml" "testdata/golden/testC/c$i-right.yaml" --no-color
done

# ---- 测试D（TOML / HCL）----
emit dyff     D-toml-dyff     "$DYFF" between --set-exit-code testdata/golden/testD/a.toml testdata/golden/testD/b.toml
emit dyff     D-hcl-dyff      "$DYFF" between --set-exit-code testdata/golden/testD/a.hcl  testdata/golden/testD/b.hcl
emit diffyml  D-toml-diffyml  "$DIFFYML" testdata/golden/testD/a.toml testdata/golden/testD/b.toml
emit diffyml  D-hcl-diffyml   "$DIFFYML" testdata/golden/testD/a.hcl  testdata/golden/testD/b.hcl
emit confdiff D-toml-confdiff "$CONFDIFF" testdata/golden/testD/a.toml testdata/golden/testD/b.toml --no-color
emit confdiff D-hcl-confdiff  "$CONFDIFF" testdata/golden/testD/a.hcl  testdata/golden/testD/b.hcl  --no-color

echo "captured under $OUT"
echo "now run: go test ./tests/ -run TestBaseline -v"
