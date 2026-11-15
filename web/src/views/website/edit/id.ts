let counter = 0

// 列表项的渲染 key，只在前端使用
export const generateId = () => `_${Date.now()}_${++counter}`
