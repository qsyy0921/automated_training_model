import { classColor, className } from "@entities/track/model";

export const ANOMALY_TYPES = [
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
  "其他异常"
] as const;

export const CLOTHING_COLORS = ["未填写", "白色", "黑色", "灰色", "蓝色", "红色", "黄色", "绿色", "橙色", "紫色", "棕色"] as const;

export const UPPER_CLOTHING = ["未填写", "短袖", "长袖", "外套", "背心", "连帽衫", "衬衫"] as const;
export const LOWER_CLOTHING = ["未填写", "长裤", "短裤", "裙子", "运动裤", "牛仔裤"] as const;
export const CARRYING = ["未填写", "背包", "手提包", "推车", "自行车", "滑板", "其他"] as const;

export { classColor, className };

