const getExt = (filename: string) => {
  const dot = filename.lastIndexOf('.')
  if (dot === -1 || dot === 0) {
    return ''
  }
  return filename.slice(dot + 1)
}

const getBase = (filename: string) => {
  const dot = filename.lastIndexOf('.')
  if (dot === -1 || dot === 0) {
    return filename
  }
  return filename.slice(0, dot)
}

const getIconByExt = (ext: string) => {
  switch (ext.toLowerCase()) {
    case 'png':
    case 'jpg':
    case 'jpeg':
    case 'gif':
    case 'webp':
    case 'svg':
      return 'mdi-file-image-outline'

    case 'mp4':
    case 'avi':
    case 'mkv':
    case 'rmvb':
    case 'mov':
      return 'mdi-file-video-outline'

    case 'mp3':
    case 'flac':
    case 'wav':
    case 'ape':
    case 'ogg':
      return 'mdi-file-music-outline'

    case 'zip':
    case 'bz2':
    case 'tar':
    case 'gz':
    case 'tgz':
    case 'xz':
    case '7z':
    case 'rar':
      return 'mdi-archive-outline'

    case 'doc':
    case 'docx':
    case 'xls':
    case 'xlsx':
    case 'ppt':
    case 'pptx':
      return 'mdi-file-document-outline'

    case 'pdf':
      return 'mdi-file-pdf-box'

    case 'txt':
    case 'md':
    case 'log':
    case 'conf':
    case 'ini':
    case 'yaml':
    case 'yml':
      return 'mdi-file-document-outline'

    case 'html':
    case 'htm':
    case 'xml':
    case 'json':
    case 'js':
    case 'css':
    case 'ts':
    case 'vue':
    case 'jsx':
    case 'tsx':
    case 'php':
    case 'java':
    case 'py':
    case 'go':
    case 'rb':
    case 'sh':
      return 'mdi-file-code-outline'

    case '':
      return 'mdi-file-outline'

    default:
      return 'mdi-file-outline'
  }
}

const languageByPath = (path: string) => {
  const ext = getExt(path)
  switch (ext) {
    case 'abap':
      return 'abap'
    case 'apex':
      return 'apex'
    case 'azcli':
      return 'azcli'
    case 'bat':
      return 'bat'
    case 'bicep':
      return 'bicep'
    case 'mligo': // cameligo 扩展名
      return 'cameligo'
    case 'clj':
    case 'cljs':
    case 'cljc': // clojure 扩展名
      return 'clojure'
    case 'coffee':
      return 'coffee'
    case 'cpp':
    case 'cc':
    case 'cxx': // cpp 扩展名
      return 'cpp'
    case 'cs':
      return 'csharp'
    case 'csp':
      return 'csp'
    case 'css':
      return 'css'
    case 'cypher':
      return 'cypher'
    case 'dart':
      return 'dart'
    case 'dockerfile':
      return 'dockerfile'
    case 'ecl':
      return 'ecl'
    case 'ex':
    case 'exs': // elixir 扩展名
      return 'elixir'
    case 'flow':
      return 'flow9'
    case 'fs':
    case 'fsi':
    case 'fsx':
    case 'fsscript': // fsharp 扩展名
      return 'fsharp'
    case 'ftl': // freemarker2 扩展名
      return 'freemarker2'
    case 'go':
      return 'go'
    case 'graphql':
      return 'graphql'
    case 'handlebars':
    case 'hbs': // handlebars 扩展名
      return 'handlebars'
    case 'hcl':
    case 'tf': // hcl 扩展名
      return 'hcl'
    case 'html':
    case 'htm': // html 扩展名
      return 'html'
    case 'ini':
    case 'conf':
      if (path.toLowerCase().includes('nginx')) {
        return 'nginx'
      }
      return 'ini'
    case 'java':
      return 'java'
    case 'js':
    case 'mjs':
    case 'cjs': // javascript 扩展名
      return 'javascript'
    case 'jl':
      return 'julia'
    case 'kt':
    case 'kts': // kotlin 扩展名
      return 'kotlin'
    case 'less':
      return 'less'
    case 'lex': // lexon 扩展名
      return 'lexon'
    case 'lua':
      return 'lua'
    case 'liquid':
      return 'liquid'
    case 'm3':
      return 'm3'
    case 'md':
      return 'markdown'
    case 'mdx':
      return 'mdx'
    case 'mips':
      return 'mips'
    case 'dax': // msdax 扩展名
      return 'msdax'
    case 'm': // objective-c 扩展名
      return 'objective-c'
    case 'pas':
      return 'pascal'
    case 'ligo': // pascaligo 扩展名
      return 'pascaligo'
    case 'pl':
      return 'perl'
    case 'php':
      return 'php'
    case 'pla':
      return 'pla'
    case 'dats':
    case 'sats':
    case 'hats': // postiats 扩展名
      return 'postiats'
    case 'pq': // powerquery 扩展名
      return 'powerquery'
    case 'ps1':
    case 'psm1': // powershell 扩展名
      return 'powershell'
    case 'proto':
      return 'protobuf'
    case 'pug':
    case 'jade': // pug 扩展名
      return 'pug'
    case 'py':
      return 'python'
    case 'qs': // qsharp 扩展名
      return 'qsharp'
    case 'r':
      return 'r'
    case 'razor':
      return 'razor'
    case 'redis':
      return 'redis'
    case 'redshift':
      return 'redshift'
    case 'rst':
      return 'restructuredtext'
    case 'rb':
      return 'ruby'
    case 'rs':
      return 'rust'
    case 'sb':
      return 'sb'
    case 'scala':
      return 'scala'
    case 'scm':
    case 'ss': // scheme 扩展名
      return 'scheme'
    case 'scss':
      return 'scss'
    case 'sh':
    case 'bash': // shell 扩展名
      return 'shell'
    case 'sol':
      return 'solidity'
    case 'sophia':
      return 'sophia'
    case 'sparql':
      return 'sparql'
    case 'sql':
      return 'sql'
    case 'st':
      return 'st'
    case 'swift':
      return 'swift'
    case 'sv':
    case 'svh': // systemverilog 扩展名
      return 'systemverilog'
    case 'tcl':
      return 'tcl'
    case 'twig':
      return 'twig'
    case 'ts':
    case 'tsx': // typescript 扩展名
      return 'typescript'
    case 'typespec':
      return 'typespec'
    case 'vb':
    case 'vbs': // vb 扩展名
      return 'vb'
    case 'wgsl':
      return 'wgsl'
    case 'xml':
    case 'xsd':
    case 'xsl':
    case 'xslt': // xml 扩展名
      return 'xml'
    case 'yaml':
    case 'yml': // yaml 扩展名
      return 'yaml'
    case 'json':
      return 'json'
    default:
      return 'plaintext'
  }
}

const checkName = (name: string) => {
  return /^[\p{L}\p{N}\p{P}\s]+$/u.test(name)
}

const checkPath = (path: string) => {
  return /^(?!\/)(?!.*\/$)(?!.*\/\/).*$/.test(path)
}

const getFilename = (path: string) => {
  const parts = path.split('/')
  return parts.pop()!
}

const isCompress = (name: string) => {
  const ext = getExt(name)
  return ['zip', 'bz2', 'tar', 'gz', 'tgz', 'xz', '7z'].includes(ext)
}

const isImage = (name: string) => {
  const ext = getExt(name)
  return [
    'png',
    'jpg',
    'jpeg',
    'gif',
    'bmp',
    'ico',
    'svg',
    'webp',
    'avif',
    'tiff',
    'heif',
    'heic',
    'jxl'
  ].includes(ext)
}

const formatPercent = (num: any) => {
  num = Number(num)
  return Number(num.toFixed(2))
}

const formatBytes = (size: any) => {
  size = Number(size)
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
  let i = 0

  while (size >= 1024 && i < units.length) {
    size /= 1024
    i++
  }

  return size.toFixed(2) + ' ' + units[i]
}

const lastDirectory = (path: string) => {
  const parts = path.split('/')
  return parts.pop() || ''
}

export {
  checkName,
  checkPath,
  formatBytes,
  formatPercent,
  getBase,
  getExt,
  getFilename,
  getIconByExt,
  isCompress,
  isImage,
  languageByPath,
  lastDirectory
}
