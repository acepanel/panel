import { http } from '@/utils'

export default {
  // 获取数据库列表
  list: (page: number, limit: number, type?: string) =>
    http.Get(`/database`, { params: { page, limit, type } }),
  // 创建数据库
  create: (data: any) => http.Post(`/database`, data),
  // 删除数据库
  delete: (server_id: number, name: string) => http.Delete(`/database`, { server_id, name }),
  // 更新评论
  comment: (server_id: number, name: string, comment: string) =>
    http.Post(`/database/comment`, { server_id, name, comment }),
  // 获取数据库服务器列表
  serverList: (page: number, limit: number, type?: string) =>
    http.Get('/database_server', { params: { page, limit, type } }),
  // 创建数据库服务器
  serverCreate: (data: any) => http.Post('/database_server', data),
  // 获取数据库服务器
  serverGet: (id: number) => http.Get(`/database_server/${id}`),
  // 更新数据库服务器
  serverUpdate: (id: number, data: any) => http.Put(`/database_server/${id}`, data),
  // 删除数据库服务器
  serverDelete: (id: number) => http.Delete(`/database_server/${id}`),
  // 同步数据库
  serverSync: (id: number) => http.Post(`/database_server/${id}/sync`),
  // 更新服务器备注
  serverRemark: (id: number, remark: string) =>
    http.Put(`/database_server/${id}/remark`, { remark }),
  // 获取数据库用户列表
  userList: (page: number, limit: number, type?: string) =>
    http.Get('/database_user', { params: { page, limit, type } }),
  // 创建数据库用户
  userCreate: (data: any) => http.Post('/database_user', data),
  // 获取数据库用户
  userGet: (id: number) => http.Get(`/database_user/${id}`),
  // 更新数据库用户
  userUpdate: (id: number, data: any) => http.Put(`/database_user/${id}`, data),
  // 删除数据库用户
  userDelete: (id: number) => http.Delete(`/database_user/${id}`),
  // 更新用户备注
  userRemark: (id: number, remark: string) => http.Put(`/database_user/${id}/remark`, { remark }),
  // Redis 获取数据库数量
  redisDatabases: (server_id: number) =>
    http.Get('/database_redis/databases', { params: { server_id } }),
  // Redis 获取 key 列表
  redisData: (server_id: number, db: number, page: number, limit: number, search?: string) =>
    http.Get('/database_redis/data', { params: { server_id, db, page, limit, search } }),
  // Redis 获取单个 key
  redisKeyGet: (server_id: number, db: number, key: string) =>
    http.Get('/database_redis/key', { params: { server_id, db, key } }),
  // Redis 设置 key
  redisKeySet: (data: any) => http.Post('/database_redis/key', data),
  // Redis 删除 key
  redisKeyDelete: (server_id: number, db: number, key: string) =>
    http.Delete('/database_redis/key', { server_id, db, key }),
  // Redis 设置 TTL
  redisKeyTTL: (server_id: number, db: number, key: string, ttl: number) =>
    http.Post('/database_redis/key/ttl', { server_id, db, key, ttl }),
  // Redis 重命名 key
  redisKeyRename: (server_id: number, db: number, old_key: string, new_key: string) =>
    http.Post('/database_redis/key/rename', { server_id, db, old_key, new_key }),
  // Redis 清空数据库
  redisClear: (server_id: number, db: number) =>
    http.Post('/database_redis/clear', { server_id, db }),
  // Elasticsearch 获取索引列表
  esIndices: (server_id: number) =>
    http.Get('/database_elasticsearch/indices', { params: { server_id } }),
  // Elasticsearch 创建索引
  esIndexCreate: (server_id: number, name: string) =>
    http.Post('/database_elasticsearch/index', { server_id, name }),
  // Elasticsearch 删除索引
  esIndexDelete: (server_id: number, name: string) =>
    http.Delete('/database_elasticsearch/index', { server_id, name }),
  // Elasticsearch 搜索文档
  esData: (server_id: number, index: string, page: number, limit: number, search?: string) =>
    http.Get('/database_elasticsearch/data', { params: { server_id, index, page, limit, search } }),
  // Elasticsearch 获取文档
  esDocumentGet: (server_id: number, index: string, id: string) =>
    http.Get('/database_elasticsearch/document', { params: { server_id, index, id } }),
  // Elasticsearch 创建/更新文档
  esDocumentSet: (data: any) => http.Post('/database_elasticsearch/document', data),
  // Elasticsearch 删除文档
  esDocumentDelete: (server_id: number, index: string, id: string) =>
    http.Delete('/database_elasticsearch/document', { server_id, index, id }),
}
