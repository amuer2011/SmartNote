const definitions = [
 ['system', '系统默认'], ['SimSun', '宋体'], ['NSimSun', '新宋体'],
 ['FangSong', '仿宋'], ['SimHei', '黑体'], ['KaiTi', '楷体'],
 ['Microsoft YaHei', '微软雅黑'], ['DengXian', '等线'],
 ['STSong', '华文宋体'], ['STFangsong', '华文仿宋'], ['STKaiti', '华文楷体'],
 ['STXihei', '华文细黑'], ['STZhongsong', '华文中宋'],
 ['FZShuTi', '方正舒体'], ['FZYaoti', '方正姚体'],
 ['Source Han Sans SC', '思源黑体'], ['Source Han Serif SC', '思源宋体'],
 ['Noto Sans CJK SC', 'Noto 黑体'], ['Noto Serif CJK SC', 'Noto 宋体'],
 ['Arial', 'Arial'], ['Times New Roman', 'Times New Roman'], ['Consolas', 'Consolas']
] as const
export const fontOptions = definitions.map(([value,label]) => ({value,label}))
export function fontStack(value?: string) {
 if (!value || value === 'system' || !fontOptions.some(option => option.value === value)) return 'Inter, "Segoe UI", "Microsoft YaHei", sans-serif'
 return `"${value}", "Microsoft YaHei", sans-serif`
}
