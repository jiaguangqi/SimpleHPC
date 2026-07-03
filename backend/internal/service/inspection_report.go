package service

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

func RenderInspectionLog(run InspectionRun) string {
	var out strings.Builder
	fmt.Fprintf(&out, "simpleHPC 集群巡检详细日志\n报告编号: %s\n集群: %s\n执行人: %s\n开始时间: %s\n结束时间: %s\n总耗时: %d ms\n\n", run.RunID, run.ClusterName, run.CreatedBy, run.StartedAt, run.FinishedAt, run.DurationMS)
	for index, check := range run.Checks {
		fmt.Fprintf(&out, "================================================================================\n[%02d/%02d] %s / %s\n状态: %s\n执行节点: %s\n命令: %s\n开始: %s\n结束: %s\n耗时: %d ms\n退出码: %d\n说明: %s\n--- STDOUT ---\n%s\n--- STDERR ---\n%s\n\n",
			index+1, len(run.Checks), check.Category, check.Name, strings.ToUpper(check.Status), check.Host, check.Command, check.StartedAt, check.FinishedAt, check.DurationMS, check.ExitCode, check.Message, check.Stdout, check.Stderr)
	}
	return out.String()
}

func summaryInt(summary map[string]any, key string) int {
	switch value := summary[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}

func summaryFloat(summary map[string]any, key string) float64 {
	switch value := summary[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func inspectionScore(run InspectionRun) int {
	total := run.PassedCount + run.ProblemCount
	if total == 0 {
		return 0
	}
	score := 100 - run.ProblemCount*12
	if score < 0 {
		return 0
	}
	return score
}

func RenderInspectionHTML(run InspectionRun) string {
	score := inspectionScore(run)
	statusText := "健康"
	if run.Status != "passed" {
		statusText = "关注"
	}
	storage := int64(0)
	switch value := run.Summary["storageBytes"].(type) {
	case int64:
		storage = value
	case float64:
		storage = int64(value)
	}
	cards := []struct{ Label, Value, Note, Icon string }{
		{"计算节点", fmt.Sprint(summaryInt(run.Summary, "nodes")), "来自 sinfo", "▦"},
		{"CPU 核心", fmt.Sprint(summaryInt(run.Summary, "cpuCores")), "Slurm 可调度核心", "◫"},
		{"GPU 数量", fmt.Sprint(summaryInt(run.Summary, "gpuCount")), "未配置时为 0", "⬡"},
		{"总内存", fmt.Sprintf("%.1f GiB", summaryFloat(run.Summary, "memoryGB")), "节点 RealMemory", "▤"},
		{"存储容量", formatBytes(storage), "配置根目录合计", "◉"},
		{"运行作业", fmt.Sprint(summaryInt(run.Summary, "runningJobs")), fmt.Sprintf("排队 %d", summaryInt(run.Summary, "pendingJobs")), "▶"},
		{"平台用户", fmt.Sprint(summaryInt(run.Summary, "users")), "未删除账户", "♙"},
		{"巡检耗时", fmt.Sprintf("%.2f s", float64(run.DurationMS)/1000), "真实命令总耗时", "◷"},
	}
	var cardHTML strings.Builder
	for _, card := range cards {
		fmt.Fprintf(&cardHTML, `<div class="metric"><div class="mhead"><span>%s</span><i>%s</i></div><strong>%s</strong><small>%s</small></div>`, html.EscapeString(card.Label), card.Icon, html.EscapeString(card.Value), html.EscapeString(card.Note))
	}
	categories := map[string][]InspectionCheck{}
	for _, check := range run.Checks {
		categories[check.Category] = append(categories[check.Category], check)
	}
	names := make([]string, 0, len(categories))
	for name := range categories {
		names = append(names, name)
	}
	sort.Strings(names)
	var detailHTML, riskHTML strings.Builder
	for _, category := range names {
		passed, skipped, failed := 0, 0, 0
		checkNames := make([]string, 0, len(categories[category]))
		for _, check := range categories[category] {
			checkNames = append(checkNames, check.Name)
			switch check.Status {
			case "ok":
				passed++
			case "skipped":
				skipped++
			default:
				failed++
				fmt.Fprintf(&riskHTML, `<tr><td><span class="level warning">警告</span></td><td>%s</td><td>%s</td><td>%s</td><td>查看详细日志并处理失败命令</td></tr>`, html.EscapeString(category), html.EscapeString(check.Name), html.EscapeString(check.Message))
			}
		}
		state := "normal"
		if failed > 0 {
			state = "warning"
		}
		fmt.Fprintf(&detailHTML, `<div class="module"><div class="module-top"><span class="module-icon">◆</span><span class="lamp %s"></span></div><small>%s</small><strong>%d / %d</strong><p>通过 %d · 跳过 %d · 异常 %d</p><p>%s</p></div>`, state, html.EscapeString(category), passed, len(categories[category]), passed, skipped, failed, html.EscapeString(strings.Join(checkNames, "、")))
	}
	if riskHTML.Len() == 0 {
		riskHTML.WriteString(`<tr><td><span class="level normal">正常</span></td><td>全部模块</td><td>未发现命令执行失败</td><td>当前集群</td><td>保持周期巡检</td></tr>`)
	}
	return fmt.Sprintf(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>HPC 集群巡检报告 %s</title>
<style>
@page{size:A3 portrait;margin:12mm}*{box-sizing:border-box}body{margin:0;background:#eaf1f8;color:#17324d;font-family:Inter,"Microsoft YaHei","PingFang SC",sans-serif}.toolbar{position:sticky;top:0;z-index:10;display:flex;justify-content:space-between;align-items:center;padding:12px calc((100%% - 1120px)/2);background:#123e85;color:#fff}.toolbar button{border:0;border-radius:8px;padding:9px 16px;background:linear-gradient(90deg,#1b71dd,#16aee2);color:white;font-weight:700;cursor:pointer}.sheet{width:min(1120px,100%%);margin:22px auto;background:#f9fcff;box-shadow:0 24px 70px rgba(35,76,117,.2);overflow:hidden}.hero{padding:40px 48px;background:radial-gradient(circle at 78%% 10%%,rgba(29,150,225,.22),transparent 28%%),linear-gradient(115deg,#e5f3ff,#fbfdff 62%%,#e4f5ff);border-bottom:1px solid #c9e1f4}.brand{display:flex;justify-content:space-between;color:#1961ac;font-size:12px;font-weight:800;letter-spacing:.1em}.hero-grid{display:grid;grid-template-columns:1fr 230px;gap:30px;align-items:center;margin-top:28px}.kicker{font-size:11px;color:#1683c9;letter-spacing:.18em}.hero h1{font-size:40px;color:#123e76;margin:8px 0}.meta{display:grid;grid-template-columns:repeat(2,1fr);gap:8px 20px;color:#6c849a;font-size:11px}.meta b{color:#284a68}.score{width:180px;height:180px;border-radius:50%%;display:grid;place-items:center;background:conic-gradient(#1b9b70 0 %d%%,#d8e7f3 %d%%);position:relative}.score:before{content:"";position:absolute;inset:10px;border-radius:50%%;background:#f8fcff}.score div{position:relative;text-align:center}.score strong{display:block;font-size:48px;color:#1767be}.score span{color:#218b68;font-weight:700}.body{padding:26px 48px 48px}.section{margin-top:22px}.heading{display:flex;justify-content:space-between;align-items:center;margin-bottom:12px}.heading h2{font-size:17px;margin:0}.heading h2:before{content:"";display:inline-block;width:5px;height:18px;margin-right:8px;border-radius:4px;background:linear-gradient(#1767d9,#15b7e5);vertical-align:middle}.heading span{font-size:9px;color:#91a8bc;letter-spacing:.16em}.metrics{display:grid;grid-template-columns:repeat(4,1fr);gap:11px}.metric{padding:15px;border-radius:13px;color:white;background:linear-gradient(135deg,#1558bd,#24a8e0);box-shadow:0 9px 20px rgba(31,100,181,.16)}.metric:nth-child(3n+2){background:linear-gradient(135deg,#176fc8,#18b0d7)}.metric:nth-child(3n+3){background:linear-gradient(135deg,#3157c2,#6882e8)}.mhead{display:flex;justify-content:space-between;font-size:11px;opacity:.9}.mhead i{font-size:18px;font-style:normal}.metric strong{display:block;font-size:24px;margin-top:7px}.metric small{opacity:.72}.summary{display:grid;grid-template-columns:1fr 1fr;gap:12px}.panel{background:white;border:1px solid #dbe9f4;border-radius:13px;padding:18px}.bars .row{display:grid;grid-template-columns:105px 1fr 40px;gap:8px;align-items:center;font-size:11px;margin:13px 0}.bar{height:7px;border-radius:8px;background:#e8f0f7}.bar i{display:block;height:100%%;border-radius:8px;background:linear-gradient(90deg,#1767d9,#16b4df)}.donut{height:180px;display:grid;place-items:center}.donut-ring{width:135px;height:135px;border-radius:50%%;background:conic-gradient(#25a878 0 %d%%,#e6a02f %d%% 100%%);display:grid;place-items:center}.donut-ring:before{content:"%d 项检查";display:grid;place-items:center;width:95px;height:95px;border-radius:50%%;background:white;font-size:13px;font-weight:700}.risk{width:100%%;border-collapse:collapse;background:white;border-radius:12px;overflow:hidden;font-size:11px}.risk th{background:#eaf4fc;color:#55738c;text-align:left;padding:11px}.risk td{border-top:1px solid #e5eef6;padding:11px}.level{padding:4px 8px;border-radius:12px;font-weight:700}.level.warning{background:#fff0d2;color:#a96906}.level.normal{background:#dbf6e9;color:#187550}.modules{display:grid;grid-template-columns:repeat(4,1fr);gap:10px}.module{background:white;border:1px solid #dce9f4;border-radius:12px;padding:14px}.module-top{display:flex;justify-content:space-between}.module-icon{display:grid;place-items:center;width:30px;height:30px;border-radius:9px;background:linear-gradient(135deg,#e0efff,#d8f7ff);color:#1674c3}.lamp{width:8px;height:8px;border-radius:50%%;background:#25aa78;box-shadow:0 0 8px #25aa78}.lamp.warning{background:#e9a02d;box-shadow:0 0 8px #e9a02d}.module small{display:block;color:#718ba2;margin-top:10px}.module strong{display:block;font-size:20px;margin-top:4px}.module p{font-size:10px;color:#8ba0b2;margin-bottom:0}.footer{display:flex;justify-content:space-between;border-top:1px solid #d6e5f1;padding:15px 48px;color:#839bad;font-size:9px}
@media print{body{background:white}.toolbar{display:none}.sheet{width:100%%;margin:0;box-shadow:none}.hero,.metric,.panel,.module{-webkit-print-color-adjust:exact;print-color-adjust:exact}.section{break-inside:avoid}}@media(max-width:800px){.hero-grid,.summary{grid-template-columns:1fr}.metrics,.modules{grid-template-columns:repeat(2,1fr)}.score{width:130px;height:130px}}
</style></head><body><div class="toolbar"><span>HPC 集群巡检报告 · %s</span><button onclick="window.print()">导出 A3 PDF</button></div><article class="sheet"><header class="hero"><div class="brand"><span>SIMPLEHPC · CLUSTER INSPECTION</span><span>REPORT NO. %s</span></div><div class="hero-grid"><div><div class="kicker">SUPERCOMPUTING CENTER · HEALTH INSPECTION</div><h1>HPC 集群巡检报告</h1><div class="meta"><span>集群名称 <b>%s</b></span><span>巡检人员 <b>%s</b></span><span>巡检时间 <b>%s</b></span><span>巡检耗时 <b>%.2f 秒</b></span></div></div><div class="score"><div><strong>%d</strong><small>/ 100</small><span>● %s</span></div></div></div></header><main class="body"><section class="section"><div class="heading"><h2>集群概览</h2><span>CLUSTER OVERVIEW</span></div><div class="metrics">%s</div></section><section class="section summary"><div class="panel"><div class="heading"><h2>健康状态总览</h2></div><div class="bars"><div class="row"><span>通过检查</span><div class="bar"><i style="width:%d%%"></i></div><b>%d</b></div><div class="row"><span>异常检查</span><div class="bar"><i style="width:%d%%;background:#e5a12e"></i></div><b>%d</b></div><div class="row"><span>环境未配置</span><div class="bar"><i style="width:%d%%;background:#7e9bb3"></i></div><b>%d</b></div></div></div><div class="panel"><div class="heading"><h2>检查结果分布</h2></div><div class="donut"><div class="donut-ring"></div></div></div></section><section class="section"><div class="heading"><h2>风险问题与处置建议</h2><span>RISKS & ACTIONS</span></div><table class="risk"><thead><tr><th>等级</th><th>模块</th><th>问题</th><th>影响</th><th>建议</th></tr></thead><tbody>%s</tbody></table></section><section class="section"><div class="heading"><h2>巡检明细</h2><span>INSPECTION DETAILS</span></div><div class="modules">%s</div></section></main><footer class="footer"><span>数据来源：真实 Slurm / LDAP / PostgreSQL / Redis / Filesystem 命令结果</span><span>报告编号 %s</span></footer></article></body></html>`,
		run.RunID, score, score, run.PassedCount*100/max(1, len(run.Checks)), run.PassedCount*100/max(1, len(run.Checks)), len(run.Checks),
		html.EscapeString(run.ClusterName), html.EscapeString(run.RunID), html.EscapeString(run.ClusterName), html.EscapeString(run.CreatedBy), html.EscapeString(run.StartedAt), float64(run.DurationMS)/1000, score, statusText, cardHTML.String(),
		run.PassedCount*100/max(1, len(run.Checks)), run.PassedCount, run.ProblemCount*100/max(1, len(run.Checks)), run.ProblemCount, run.SkippedCount*100/max(1, len(run.Checks)), run.SkippedCount,
		riskHTML.String(), detailHTML.String(), html.EscapeString(run.RunID))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
