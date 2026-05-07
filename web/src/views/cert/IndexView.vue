<script setup lang="ts">
defineOptions({
  name: 'cert-index',
})

import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import cert from '@/api/panel/cert'
import website from '@/api/panel/website'
import AccountView from '@/views/cert/AccountView.vue'
import CertView from '@/views/cert/CertView.vue'
import CreateAccountModal from '@/views/cert/CreateAccountModal.vue'
import CreateCertModal from '@/views/cert/CreateCertModal.vue'
import CreateDnsModal from '@/views/cert/CreateDnsModal.vue'
import DnsView from '@/views/cert/DnsView.vue'
import UploadCertModal from '@/views/cert/UploadCertModal.vue'

const { $gettext } = useGettext()
const currentTab = ref('cert')

const uploadCert = ref(false)
const createCert = ref(false)
const createDNS = ref(false)
const createAccount = ref(false)

const algorithms = ref<any>([])
const websites = ref<any>([])
const dns = ref<any>([])
const accounts = ref<any>([])
const dnsProviders = ref<any>([])
const caProviders = ref<any>([])

const getAsyncData = () => {
  useRequest(cert.algorithms()).onSuccess(({ data }) => {
    algorithms.value = data
  })

  websites.value = []
  useRequest(app.isInstalled('nginx,openresty,apache,caddy')).onSuccess(({ data }) => {
    if (data) {
      useRequest(website.list('all', 1, 10000)).onSuccess(({ data }) => {
        for (const item of data.items) {
          websites.value.push({
            label: item.name,
            value: item.id,
          })
        }
      })
    }
  })

  dns.value = []
  useRequest(cert.dns(1, 10000)).onSuccess(({ data }) => {
    for (const item of data.items) {
      dns.value.push({
        label: item.name,
        value: item.id,
      })
    }
  })

  useRequest(cert.dnsProviders()).onSuccess(({ data }) => {
    dnsProviders.value = data
  })
  useRequest(cert.caProviders()).onSuccess(({ data }) => {
    caProviders.value = data
    accounts.value = []
    useRequest(cert.accounts(1, 10000)).onSuccess(({ data }) => {
      for (const item of data.items) {
        const caLabel = caProviders.value?.find((ca: any) => ca.value === item.ca)?.label
        accounts.value.push({
          label: `${item.email} (${caLabel ?? item.ca})`,
          value: item.id,
        })
      }
    })
  })
}

onMounted(() => {
  getAsyncData()
  window.$bus.on('cert:refresh-async', getAsyncData)
})

onUnmounted(() => {
  window.$bus.off('cert:refresh-async')
})
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-model:value="currentTab" animated>
        <n-tab name="cert" :tab="$gettext('Certificate')" />
        <n-tab name="account" :tab="$gettext('Account')" />
        <n-tab name="dns" :tab="$gettext('DNS')" />
      </n-tabs>
    </template>
    <n-flex vertical>
      <n-flex>
        <n-button v-if="currentTab == 'cert'" type="success" @click="uploadCert = true">
          {{ $gettext('Upload Certificate') }}
        </n-button>
        <n-button v-if="currentTab == 'cert'" type="primary" @click="createCert = true">
          {{ $gettext('Create Certificate') }}
        </n-button>
        <n-button v-if="currentTab == 'account'" type="primary" @click="createAccount = true">
          {{ $gettext('Create Account') }}
        </n-button>
        <n-button v-if="currentTab == 'dns'" type="primary" @click="createDNS = true">
          {{ $gettext('Create DNS') }}
        </n-button>
      </n-flex>
      <cert-view
        v-if="currentTab == 'cert'"
        :accounts="accounts"
        :algorithms="algorithms"
        :websites="websites"
        :dns="dns"
      />
      <account-view
        v-if="currentTab == 'account'"
        :ca-providers="caProviders"
        :algorithms="algorithms"
      />
      <dns-view v-if="currentTab == 'dns'" :dns-providers="dnsProviders" />
    </n-flex>
  </PageContainer>
  <upload-cert-modal v-model:show="uploadCert" />
  <create-cert-modal
    v-model:show="createCert"
    :accounts="accounts"
    :algorithms="algorithms"
    :websites="websites"
    :dns="dns"
  />
  <create-dns-modal v-model:show="createDNS" :dns-providers="dnsProviders" />
  <create-account-modal
    v-model:show="createAccount"
    :ca-providers="caProviders"
    :algorithms="algorithms"
  />
</template>
