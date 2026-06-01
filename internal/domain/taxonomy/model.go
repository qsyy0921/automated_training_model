package taxonomy

type Taxonomy struct {
	AnomalyTypes           []string `json:"anomaly_types"`
	ClothingColors         []string `json:"clothing_colors"`
	UpperClothing          []string `json:"upper_clothing"`
	LowerClothing          []string `json:"lower_clothing"`
	Carrying               []string `json:"carrying"`
	TrackingStatuses       []string `json:"tracking_statuses"`
	TrackingRejectStatuses []string `json:"tracking_reject_statuses"`
}

func Default() Taxonomy {
	return Taxonomy{
		AnomalyTypes: []string{
			"无",
			"骑车",
			"跑步",
			"滑板",
			"车辆进入",
			"摩托车",
			"打斗",
			"摔倒",
			"跳跃",
			"徘徊",
			"投掷",
			"婴儿车",
			"其他异常",
		},
		ClothingColors: []string{"未填写", "白色", "黑色", "灰色", "蓝色", "红色", "黄色", "绿色", "橙色", "紫色", "棕色"},
		UpperClothing:  []string{"未填写", "短袖", "长袖", "外套", "背心", "连帽衫", "衬衣"},
		LowerClothing:  []string{"未填写", "长裤", "短裤", "裙子", "运动裤", "牛仔裤"},
		Carrying:       []string{"未填写", "背包", "手提包", "推车", "自行车", "滑板", "其他"},
		TrackingStatuses: []string{
			"unchecked",
			"accepted",
			"rejected",
			"通过",
			"删除",
		},
		TrackingRejectStatuses: []string{
			"rejected",
			"reject",
			"delete",
			"deleted",
			"删除",
		},
	}
}

func (t Taxonomy) FillDefaults() Taxonomy {
	defaults := Default()
	if len(t.AnomalyTypes) == 0 {
		t.AnomalyTypes = defaults.AnomalyTypes
	}
	if len(t.ClothingColors) == 0 {
		t.ClothingColors = defaults.ClothingColors
	}
	if len(t.UpperClothing) == 0 {
		t.UpperClothing = defaults.UpperClothing
	}
	if len(t.LowerClothing) == 0 {
		t.LowerClothing = defaults.LowerClothing
	}
	if len(t.Carrying) == 0 {
		t.Carrying = defaults.Carrying
	}
	if len(t.TrackingStatuses) == 0 {
		t.TrackingStatuses = defaults.TrackingStatuses
	}
	if len(t.TrackingRejectStatuses) == 0 {
		t.TrackingRejectStatuses = defaults.TrackingRejectStatuses
	}
	return t
}
