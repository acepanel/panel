export interface S3Provider {
  key: string
  name: string
  // 不含 bucket 的端点主机名，{region} 为地域 ID
  endpoint: string
  style: 'virtual-hosted' | 'path'
  regions: string[]
  bucketHint?: string
  // 控制台里两个密钥的叫法，是各家的产品术语，不翻译
  keys: [string, string]
}

// 预置负责填好的四个字段，与后端 BackupStorageInfo 的同名字段一一对应
export interface S3Preset {
  style: string
  region: string
  endpoint: string
  scheme: string
}

// 只用来让提取工具识别 msgid，翻译在表单渲染时做
const $gettext = (msgid: string) => msgid

// 端点与地域均取自各家官方文档的 S3 兼容接口说明；收录的服务商签名 region 都等于地域 ID
export const s3Providers: S3Provider[] = [
  {
    key: 'aliyun',
    keys: ['AccessKey ID', 'AccessKey Secret'],
    name: $gettext('Alibaba Cloud OSS'),
    endpoint: 's3.oss-{region}.aliyuncs.com',
    style: 'virtual-hosted',
    regions: [
      'cn-hangzhou',
      'cn-shanghai',
      'cn-wuhan-lr',
      'cn-qingdao',
      'cn-beijing',
      'cn-zhangjiakou',
      'cn-huhehaote',
      'cn-wulanchabu',
      'cn-shenzhen',
      'cn-heyuan',
      'cn-guangzhou',
      'cn-chengdu',
      'cn-zhongwei',
      'cn-hongkong',
      'ap-northeast-1',
      'ap-northeast-2',
      'ap-southeast-1',
      'ap-southeast-3',
      'ap-southeast-5',
      'ap-southeast-6',
      'ap-southeast-7',
      'ap-southeast-8',
      'eu-central-1',
      'eu-west-1',
      'us-east-1',
      'us-west-1',
      'na-south-1',
      'me-east-1',
    ],
  },
  {
    key: 'tencent',
    keys: ['SecretId', 'SecretKey'],
    name: $gettext('Tencent Cloud COS'),
    endpoint: 'cos.{region}.myqcloud.com',
    style: 'virtual-hosted',
    regions: [
      'ap-beijing',
      'ap-nanjing',
      'ap-shanghai',
      'ap-guangzhou',
      'ap-chengdu',
      'ap-chongqing',
      'ap-beijing-1',
      'ap-hongkong',
      'ap-singapore',
      'ap-jakarta',
      'ap-seoul',
      'ap-bangkok',
      'ap-tokyo',
      'na-siliconvalley',
      'na-ashburn',
      'sa-saopaulo',
      'eu-frankfurt',
      'me-saudi-arabia',
    ],
    bucketHint: $gettext('The bucket name must include the APPID suffix, e.g. backup-1250000000'),
  },
  {
    key: 'huawei',
    keys: ['Access Key ID', 'Secret Access Key'],
    name: $gettext('Huawei Cloud OBS'),
    endpoint: 'obs.{region}.myhuaweicloud.com',
    style: 'virtual-hosted',
    regions: [
      'cn-north-4',
      'cn-north-1',
      'cn-north-2',
      'cn-north-9',
      'cn-north-12',
      'cn-east-3',
      'cn-east-2',
      'cn-east-4',
      'cn-east-5',
      'cn-south-1',
      'cn-south-2',
      'cn-southwest-2',
      'ap-southeast-1',
      'ap-southeast-2',
      'ap-southeast-3',
      'ap-southeast-4',
      'ap-southeast-5',
      'af-north-1',
      'af-south-1',
      'la-north-2',
      'la-south-2',
      'na-mexico-1',
      'sa-brazil-1',
      'sa-peru-1',
      'me-east-1',
      'tr-west-1',
      'ru-northwest-2',
    ],
  },
  {
    key: 'qiniu',
    keys: ['AccessKey', 'SecretKey'],
    name: $gettext('Qiniu Cloud Kodo'),
    endpoint: 's3.{region}.qiniucs.com',
    style: 'virtual-hosted',
    regions: [
      'cn-east-1',
      'cn-east-2',
      'cn-north-1',
      'cn-south-1',
      'us-north-1',
      'ap-southeast-1',
      'ap-southeast-2',
      'ap-southeast-3',
    ],
    bucketHint: $gettext('Use the S3 bucket name shown on the bucket overview page'),
  },
  {
    key: 'baidu',
    keys: ['Access Key ID', 'Secret Access Key'],
    name: $gettext('Baidu AI Cloud BOS'),
    endpoint: 's3.{region}.bcebos.com',
    style: 'virtual-hosted',
    regions: ['bj', 'gz', 'su', 'bd'],
  },
  {
    key: 'volcengine',
    keys: ['Access Key ID', 'Secret Access Key'],
    name: $gettext('Volcengine TOS'),
    endpoint: 'tos-s3-{region}.volces.com',
    style: 'virtual-hosted',
    regions: [
      'cn-beijing',
      'cn-shanghai',
      'cn-guangzhou',
      'cn-hongkong',
      'ap-southeast-1',
      'ap-southeast-3',
    ],
  },
  {
    key: 'jdcloud',
    keys: ['AccessKey ID', 'AccessKey Secret'],
    name: $gettext('JD Cloud OSS'),
    endpoint: 's3.{region}.jdcloud-oss.com',
    style: 'virtual-hosted',
    regions: ['cn-north-1', 'cn-east-1', 'cn-east-2', 'cn-south-1', 'eu-west-1'],
  },
  {
    key: 'aws',
    keys: ['Access Key ID', 'Secret Access Key'],
    name: $gettext('Amazon S3'),
    endpoint: 's3.{region}.amazonaws.com',
    style: 'virtual-hosted',
    regions: [
      'us-east-1',
      'us-east-2',
      'us-west-1',
      'us-west-2',
      'af-south-1',
      'ap-east-1',
      'ap-east-2',
      'ap-northeast-1',
      'ap-northeast-2',
      'ap-northeast-3',
      'ap-south-1',
      'ap-south-2',
      'ap-southeast-1',
      'ap-southeast-2',
      'ap-southeast-3',
      'ap-southeast-4',
      'ap-southeast-5',
      'ap-southeast-6',
      'ap-southeast-7',
      'ca-central-1',
      'ca-west-1',
      'eu-central-1',
      'eu-central-2',
      'eu-north-1',
      'eu-south-1',
      'eu-south-2',
      'eu-west-1',
      'eu-west-2',
      'eu-west-3',
      'il-central-1',
      'me-central-1',
      'mx-central-1',
      'sa-east-1',
    ],
  },
  {
    key: 'aws-cn',
    keys: ['Access Key ID', 'Secret Access Key'],
    name: $gettext('Amazon S3 (China)'),
    endpoint: 's3.{region}.amazonaws.com.cn',
    style: 'virtual-hosted',
    regions: ['cn-north-1', 'cn-northwest-1'],
  },
]

export function endpointOf(provider: S3Provider, region: string) {
  return provider.endpoint.replace('{region}', region)
}

export function presetOf(provider: S3Provider, region: string): S3Preset {
  return { style: provider.style, region, endpoint: endpointOf(provider, region), scheme: 'https' }
}

// 由端点反推服务商与地域。四个字段都得与预置算出来的一致才算命中，
// 否则编辑一个手填的存储会在保存时被预置悄悄改掉 region 或 scheme
export function matchProvider(info: S3Preset) {
  for (const provider of s3Providers) {
    const pattern = provider.endpoint.replace(/[.]/g, '\\.').replace('{region}', '([a-z0-9-]+)')
    const region = new RegExp(`^${pattern}$`).exec(info.endpoint)?.[1]
    if (!region) continue
    const preset = presetOf(provider, region)
    if (
      preset.style === info.style &&
      preset.region === info.region &&
      preset.scheme === info.scheme
    ) {
      return { provider, region }
    }
  }
}
