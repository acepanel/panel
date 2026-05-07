import { http } from '@/utils'

export default {
  pingStatus: (): any => http.Get('/safe/ping'),
  updatePingStatus: (status: boolean): any => http.Post('/safe/ping', { status }),
}
