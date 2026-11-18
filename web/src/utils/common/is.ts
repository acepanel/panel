/** null or undefined */
export function isNullOrUndef(val: any): boolean {
  return val === null || val === undefined
}

export function isUrl(path: string): boolean {
  const reg =
    /(((^https?:(?:\/\/)?)(?:[-;:&=+$,\w]+@)?[A-Za-z0-9.-]+(?::\d+)?|(?:www.|[-;:&=+$,\w]+@)[A-Za-z0-9.-]+)((?:\/[+~%/.\w-]*)?\??[-+=&;%@.\w]*(?:#\w*)?)?)$/
  return reg.test(path)
}

export function isThenable(val: unknown): val is PromiseLike<unknown> {
  return typeof (val as PromiseLike<unknown> | undefined)?.then === 'function'
}
