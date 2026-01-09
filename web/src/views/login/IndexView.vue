<script lang="ts" setup>
import user from '@/api/panel/user'
import bgImg from '@/assets/images/login_bg.webp'
import logoImg from '@/assets/images/logo.png'
import { addDynamicRoutes } from '@/router'
import { useThemeStore, useUserStore } from '@/store'
import { getLocal, removeLocal, setLocal } from '@/utils'
import { rsaEncrypt } from '@/utils/encrypt'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const router = useRouter()
const route = useRoute()
const query = route.query
const { data: key, loading: isLoading } = useRequest(user.key, { initialData: '' })
const { data: isLogin } = useRequest(user.isLogin, { initialData: false })

interface LoginInfo {
  username: string
  password: string
  safe_login: boolean
  pass_code: string
  captcha_code: string
}

const loginInfo = ref<LoginInfo>({
  username: '',
  password: '',
  safe_login: true,
  pass_code: '',
  captcha_code: ''
})

const localLoginInfo = getLocal('loginInfo') as LoginInfo
if (localLoginInfo) {
  loginInfo.value.username = localLoginInfo.username || ''
  loginInfo.value.password = localLoginInfo.password || ''
}

const userStore = useUserStore()
const themeStore = useThemeStore()
const logining = ref<boolean>(false)
const isRemember = useStorage('isRemember', false)
const showTwoFA = ref(false)

// 验证码相关
const captchaRequired = ref(false)
const captchaImage = ref('')

const logo = computed(() => themeStore.logo || logoImg)

// 刷新验证码
const refreshCaptcha = () => {
  useRequest(user.captcha())
    .onSuccess(({ data }) => {
      captchaRequired.value = Boolean(data.required)
      captchaImage.value = data.image || ''
      loginInfo.value.captcha_code = ''
    })
    .onError(() => {
      captchaRequired.value = false
      captchaImage.value = ''
    })
}

// 初始加载验证码
onMounted(() => {
  refreshCaptcha()
})

async function handleLogin() {
  const { username, password, pass_code, safe_login, captcha_code } = loginInfo.value
  if (!username || !password) {
    window.$message.warning($gettext('Please enter username and password'))
    return
  }
  const trimmedCaptcha = captcha_code?.trim() || ''
  if (captchaRequired.value && !trimmedCaptcha) {
    window.$message.warning($gettext('Please enter captcha code'))
    return
  }
  if (!key) {
    window.$message.warning(
      $gettext('Failed to get encryption public key, please refresh the page and try again')
    )
    return
  }
  useRequest(
    user.login(
      rsaEncrypt(username, String(unref(key))),
      rsaEncrypt(password, String(unref(key))),
      pass_code,
      safe_login,
      trimmedCaptcha
    )
  )
    .onSuccess(async () => {
      logining.value = true
      window.$notification?.success({ title: $gettext('Login successful!'), duration: 2500 })
      if (isRemember.value) {
        setLocal('loginInfo', { username, password })
      } else {
        removeLocal('loginInfo')
      }

      await addDynamicRoutes()
      useRequest(user.info()).onSuccess(({ data }) => {
        userStore.set(data as any)
      })
      if (query.redirect) {
        const path = query.redirect as string
        Reflect.deleteProperty(query, 'redirect')
        await router.push({ path, query })
      } else {
        await router.push('/')
      }
    })
    .onError(() => {
      // 登录失败后刷新验证码状态
      refreshCaptcha()
    })
  logining.value = false
}

const isTwoFA = () => {
  const { username } = loginInfo.value
  if (!username) {
    return
  }
  useRequest(user.isTwoFA(username))
    .onSuccess(({ data }) => {
      showTwoFA.value = Boolean(data)
    })
    .onError(() => {
      showTwoFA.value = false
    })
}

watch(isLogin, async () => {
  if (isLogin) {
    await addDynamicRoutes()
    useRequest(user.info()).onSuccess(({ data }) => {
      userStore.set(data as any)
    })
    if (query.redirect) {
      const path = query.redirect as string
      Reflect.deleteProperty(query, 'redirect')
      await router.push({ path, query })
    } else {
      await router.push('/')
    }
  }
})
</script>

<template>
  <AppPage :show-footer="true" :style="{ backgroundImage: `url(${bgImg})` }" bg-cover>
    <div m-auto p-15 bg-white bg-opacity-60 f-c-c min-w-345 card-shadow dark:bg-dark>
      <div px-20 py-35 flex-col w-480>
        <h5 color="#6a6a6a" text-24 font-normal f-c-c>
          <n-image :src="logo" preview-disabled mr-10 h-48 />{{ themeStore.name }}
        </h5>
        <div mt-30>
          <n-input
            v-model:value="loginInfo.username"
            :maxlength="32"
            autofocus
            class="text-16 pl-10 h-50 items-center"
            :placeholder="$gettext('Username')"
            :on-blur="isTwoFA"
          />
        </div>
        <div mt-30>
          <n-input
            v-model:value="loginInfo.password"
            :maxlength="32"
            class="text-16 pl-10 h-50 items-center"
            :placeholder="$gettext('Password')"
            type="password"
            show-password-on="click"
            @keydown.enter="handleLogin"
          />
        </div>
        <div v-if="showTwoFA" mt-30>
          <n-input
            v-model:value="loginInfo.pass_code"
            :maxlength="6"
            class="text-16 pl-10 h-50 items-center"
            :placeholder="$gettext('2FA Code')"
            type="text"
            @keydown.enter="handleLogin"
          />
        </div>
        <div v-if="captchaRequired" mt-30>
          <n-flex align="center">
            <n-input
              v-model:value="loginInfo.captcha_code"
              :maxlength="4"
              class="text-16 pl-10 h-50 items-center"
              style="flex: 1"
              :placeholder="$gettext('Captcha Code')"
              type="text"
              @keydown.enter="handleLogin"
            />
            <n-image
              :src="'data:image/png;base64,' + captchaImage"
              preview-disabled
              class="cursor-pointer h-50"
              style="border-radius: 4px"
              @click="refreshCaptcha"
            />
          </n-flex>
        </div>

        <div mt-20>
          <n-flex>
            <n-checkbox v-model:checked="loginInfo.safe_login" :label="$gettext('Safe Login')" />
            <n-checkbox v-model:checked="isRemember" :label="$gettext('Remember Me')" />
          </n-flex>
        </div>

        <div mt-20>
          <n-button
            :loading="isLoading || logining"
            :disabled="isLoading || logining"
            type="primary"
            text-16
            h-50
            w-full
            @click="handleLogin"
          >
            {{ $gettext('Login') }}
          </n-button>
        </div>
      </div>
    </div>
  </AppPage>
</template>
