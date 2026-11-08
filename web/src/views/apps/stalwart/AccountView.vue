<script setup lang="ts">
defineOptions({
  name: 'stalwart-accounts',
})

import { NButton, NDataTable, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import stalwart from '@/api/apps/stalwart'
import ListInput from '@/components/common/ListInput.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { useTablePagination } from '@/components/system/composables/useTablePagination'
import { formatBytes, formatDateTime, generateRandomString } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const formLayout = useFormLayout()

const MB = 1024 * 1024

const domains = ref<any[]>([])
const domainID = ref<string | null>(null)
const keyword = ref('')

const addModal = ref(false)
const addLoading = ref(false)
const addModel = ref({
  name: '',
  domain_id: '',
  password: generateRandomString(16),
  description: '',
  quota: 0,
  aliases: [] as string[],
})

const editModal = ref(false)
const editLoading = ref(false)
const editModel = ref({
  id: '',
  email: '',
  description: '',
  quota: 0,
  aliases: [] as string[],
})

const passwordModal = ref(false)
const passwordLoading = ref(false)
const passwordModel = ref({
  id: '',
  email: '',
  password: generateRandomString(16),
})

const domainOptions = computed(() =>
  domains.value.map((d: any) => ({ label: d.name, value: d.id })),
)

const { loading, data, page, total, pageSize, refresh } = usePagination(
  (page, pageSize) => stalwart.accounts(page, pageSize, domainID.value ?? '', keyword.value),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
    watchingStates: [domainID],
  },
)
const pagination = useTablePagination(page, pageSize, total)

const columns: any = [
  {
    title: $gettext('Email'),
    key: 'email',
    minWidth: 240,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Description'),
    key: 'description',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render: (row: any) => row.description || '-',
  },
  {
    title: $gettext('Quota'),
    key: 'quota',
    width: 180,
    ellipsis: { tooltip: true },
    render(row: any) {
      const used = formatBytes(row.used)
      return row.quota
        ? `${used} / ${formatBytes(row.quota)}`
        : `${used} / ${$gettext('Unlimited')}`
    },
  },
  {
    title: $gettext('Aliases'),
    key: 'aliases',
    minWidth: 220,
    resizable: true,
    render(row: any) {
      if (!row.aliases?.length) return '-'
      return h(NFlex, { size: 'small' }, () =>
        row.aliases.map((alias: string) =>
          h(NTag, { size: 'small', bordered: false }, { default: () => alias }),
        ),
      )
    },
  },
  {
    title: $gettext('Created At'),
    key: 'created_at',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row: any) => formatDateTime(row.created_at),
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 300,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center', wrap: false }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            secondary: true,
            onClick: () => {
              editModel.value = {
                id: row.id,
                email: row.email,
                description: row.description,
                quota: Math.round(row.quota / MB),
                aliases: [...row.aliases],
              }
              editModal.value = true
            },
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            secondary: true,
            onClick: () => {
              passwordModel.value = {
                id: row.id,
                email: row.email,
                password: generateRandomString(16),
              }
              passwordModal.value = true
            },
          },
          { default: () => $gettext('Change Password') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext(
                  'Are you sure you want to delete mailbox %{ email }? All of its mail will be deleted.',
                  { email: row.email },
                ),
              })
              if (ok) handleDelete(row.id)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const loadDomains = async () => {
  const res = await stalwart.domains(1, 10000)
  domains.value = res.items
  if (!addModel.value.domain_id) {
    addModel.value.domain_id = domains.value[0]?.id ?? ''
  }
}

const cleanAliases = (aliases: string[]) => aliases.map((a) => a.trim()).filter(Boolean)

const handleAdd = () => {
  addLoading.value = true
  useRequest(
    stalwart.accountCreate({
      name: addModel.value.name,
      domain_id: addModel.value.domain_id,
      password: addModel.value.password,
      description: addModel.value.description,
      quota: addModel.value.quota * MB,
      aliases: cleanAliases(addModel.value.aliases),
    }),
  )
    .onSuccess(() => {
      refresh()
      addModal.value = false
      addModel.value.name = ''
      addModel.value.password = generateRandomString(16)
      addModel.value.description = ''
      addModel.value.quota = 0
      addModel.value.aliases = []
      window.$message.success($gettext('Added successfully'))
    })
    .onComplete(() => {
      addLoading.value = false
    })
}

const handleEdit = () => {
  editLoading.value = true
  useRequest(
    stalwart.accountUpdate(editModel.value.id, {
      description: editModel.value.description,
      quota: editModel.value.quota * MB,
      aliases: cleanAliases(editModel.value.aliases),
    }),
  )
    .onSuccess(() => {
      refresh()
      editModal.value = false
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      editLoading.value = false
    })
}

const handlePassword = () => {
  passwordLoading.value = true
  useRequest(stalwart.accountPassword(passwordModel.value.id, passwordModel.value.password))
    .onSuccess(() => {
      passwordModal.value = false
      window.$message.success($gettext('Modified successfully'))
    })
    .onComplete(() => {
      passwordLoading.value = false
    })
}

const handleDelete = (id: string) => {
  useRequest(stalwart.accountDelete(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

onMounted(() => {
  loadDomains()
  refresh()
})
</script>

<template>
  <n-flex vertical>
    <n-flex align="center">
      <n-button type="primary" @click="addModal = true">
        {{ $gettext('Add Mailbox') }}
      </n-button>
      <n-select
        v-model:value="domainID"
        class="w-60"
        clearable
        :options="domainOptions"
        :placeholder="$gettext('All domains')"
      />
      <n-input
        v-model:value="keyword"
        class="w-60"
        clearable
        :placeholder="$gettext('Search')"
        @keyup.enter="refresh"
        @clear="refresh"
      />
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1300"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>
  <app-modal v-model:show="addModal" :title="$gettext('Add Mailbox')">
    <n-flex vertical>
      <n-form :model="addModel" v-bind="formLayout">
        <n-form-item path="name" :label="$gettext('Username')">
          <n-input-group>
            <n-input v-model:value="addModel.name" placeholder="user" @keydown.enter.prevent />
            <n-input-group-label>@</n-input-group-label>
            <n-select
              v-model:value="addModel.domain_id"
              class="w-60"
              :options="domainOptions"
              :placeholder="$gettext('Select domain')"
            />
          </n-input-group>
        </n-form-item>
        <n-form-item path="password" :label="$gettext('Password')">
          <n-input
            v-model:value="addModel.password"
            type="password"
            show-password-on="click"
            @keydown.enter.prevent
            :placeholder="
              $gettext('It is recommended to use the generator to generate a random password')
            "
          />
        </n-form-item>
        <n-form-item path="description" :label="$gettext('Description')">
          <n-input
            v-model:value="addModel.description"
            @keydown.enter.prevent
            :placeholder="$gettext('Optional')"
          />
        </n-form-item>
        <n-form-item path="quota" :label="$gettext('Quota (MB)')">
          <n-input-number
            v-model:value="addModel.quota"
            class="w-full"
            :min="0"
            :step="1024"
            @keydown.enter.prevent
          >
            <template #suffix>MB</template>
          </n-input-number>
        </n-form-item>
        <n-form-item path="aliases" :label="$gettext('Aliases')">
          <ListInput v-model:value="addModel.aliases" placeholder="alias@example.com" />
        </n-form-item>
      </n-form>
      <n-text depth="3">
        {{
          $gettext(
            'Quota 0 means unlimited. Aliases must be full addresses under a domain managed here.',
          )
        }}
      </n-text>
      <n-button type="info" block :loading="addLoading" :disabled="addLoading" @click="handleAdd">
        {{ $gettext('Submit') }}
      </n-button>
    </n-flex>
  </app-modal>
  <app-modal v-model:show="editModal" :title="$gettext('Edit Mailbox')">
    <n-flex vertical>
      <n-form :model="editModel" v-bind="formLayout">
        <n-form-item :label="$gettext('Email')">
          <n-input :value="editModel.email" disabled />
        </n-form-item>
        <n-form-item path="description" :label="$gettext('Description')">
          <n-input
            v-model:value="editModel.description"
            @keydown.enter.prevent
            :placeholder="$gettext('Optional')"
          />
        </n-form-item>
        <n-form-item path="quota" :label="$gettext('Quota (MB)')">
          <n-input-number
            v-model:value="editModel.quota"
            class="w-full"
            :min="0"
            :step="1024"
            @keydown.enter.prevent
          >
            <template #suffix>MB</template>
          </n-input-number>
        </n-form-item>
        <n-form-item path="aliases" :label="$gettext('Aliases')">
          <ListInput v-model:value="editModel.aliases" placeholder="alias@example.com" />
        </n-form-item>
      </n-form>
      <n-button
        type="info"
        block
        :loading="editLoading"
        :disabled="editLoading"
        @click="handleEdit"
      >
        {{ $gettext('Save') }}
      </n-button>
    </n-flex>
  </app-modal>
  <app-modal v-model:show="passwordModal" :title="$gettext('Change Password')">
    <n-flex vertical>
      <n-form :model="passwordModel" v-bind="formLayout">
        <n-form-item :label="$gettext('Email')">
          <n-input :value="passwordModel.email" disabled />
        </n-form-item>
        <n-form-item path="password" :label="$gettext('Password')">
          <n-input
            v-model:value="passwordModel.password"
            type="text"
            @keydown.enter.prevent
            :placeholder="
              $gettext('It is recommended to use the generator to generate a random password')
            "
          />
        </n-form-item>
      </n-form>
      <n-text depth="3">
        {{
          $gettext(
            'Changing the password signs out all clients and removes app passwords of this mailbox.',
          )
        }}
      </n-text>
      <n-button
        type="info"
        block
        :loading="passwordLoading"
        :disabled="passwordLoading"
        @click="handlePassword"
      >
        {{ $gettext('Submit') }}
      </n-button>
    </n-flex>
  </app-modal>
</template>
