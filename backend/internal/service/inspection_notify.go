package service

import "fmt"

func feishuText(text string) map[string]any {
	return map[string]any{"tag": "text", "text": text}
}

func BuildInspectionFeishuPost(run InspectionRun, reportURL string) map[string]any {
	status := "✅ 健康"
	if run.Status != "passed" {
		status = "⚠️ 需要关注"
	}
	summary := run.Summary
	rows := [][]map[string]any{
		{feishuText(fmt.Sprintf("集群：%s\n", run.ClusterName)), feishuText(fmt.Sprintf("状态：%s　健康评分：%d / 100", status, inspectionScore(run)))},
		{feishuText(fmt.Sprintf("节点：%d　CPU：%d 核　GPU：%d　内存：%.1f GiB", summaryInt(summary, "nodes"), summaryInt(summary, "cpuCores"), summaryInt(summary, "gpuCount"), summaryFloat(summary, "memoryGB")))},
		{feishuText(fmt.Sprintf("存储：%s　运行作业：%d　排队作业：%d　平台用户：%d", formatBytes(int64(summaryFloat(summary, "storageBytes"))), summaryInt(summary, "runningJobs"), summaryInt(summary, "pendingJobs"), summaryInt(summary, "users")))},
		{feishuText(fmt.Sprintf("检查结果：通过 %d　异常 %d　跳过 %d　耗时 %.2f 秒", run.PassedCount, run.ProblemCount, run.SkippedCount, float64(run.DurationMS)/1000))},
	}
	for _, check := range run.Checks {
		if check.Status == "ok" {
			continue
		}
		prefix := "⚠️"
		if check.Status == "skipped" {
			prefix = "ℹ️"
		}
		rows = append(rows, []map[string]any{feishuText(fmt.Sprintf("%s [%s] %s：%s", prefix, check.Category, check.Name, check.Message))})
		if len(rows) >= 12 {
			break
		}
	}
	rows = append(rows, []map[string]any{
		{"tag": "a", "text": "在线查看完整报告", "href": reportURL},
		feishuText("　｜　报告编号：" + run.RunID),
	})
	return map[string]any{
		"msg_type": "post",
		"content": map[string]any{
			"post": map[string]any{
				"zh_cn": map[string]any{
					"title":   "simpleHPC · HPC 集群巡检报告",
					"content": rows,
				},
			},
		},
	}
}
