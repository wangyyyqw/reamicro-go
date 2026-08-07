<script lang="ts" setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { SendEmailCode, Login, Logout, GetSession, SetBooksDir, ScanBooks, StartHost, StopHost, GetHostStatus, ChooseBooksDir } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'

interface Session {
  loggedIn: boolean
  userId: number
  nickname: string
  email: string
  booksDir: string
}
interface BookInfo {
  uuid: string
  title: string
  size: number
}
interface HostStatus {
  running: boolean
  port: number
  deviceId: string
  nickname: string
}

const view = ref<'login' | 'main'>('login')
const session = ref<Session>({ loggedIn: false, userId: 0, nickname: '', email: '', booksDir: '' })

// 登录
const email = ref('')
const code = ref('')
const loginMsg = ref('')
const loginBusy = ref(false)

async function sendCode() {
  loginMsg.value = ''
  if (!email.value.trim()) { loginMsg.value = '请输入邮箱'; return }
  loginBusy.value = true
  try {
    await SendEmailCode(email.value.trim())
    loginMsg.value = '验证码已发送，请查收邮箱'
  } catch (e: any) {
    loginMsg.value = '发送失败: ' + (e?.message || e)
  } finally {
    loginBusy.value = false
  }
}

async function doLogin() {
  loginMsg.value = ''
  if (!email.value.trim() || !code.value.trim()) { loginMsg.value = '请输入邮箱和验证码'; return }
  loginBusy.value = true
  try {
    const res = await Login(email.value.trim(), code.value.trim())
    session.value.loggedIn = true
    session.value.userId = res.userId
    session.value.nickname = res.nickname
    session.value.email = res.email
    await loadSession()
    view.value = 'main'
  } catch (e: any) {
    loginMsg.value = '登录失败: ' + (e?.message || e)
  } finally {
    loginBusy.value = false
  }
}

// 主界面
const booksDir = ref('')
const books = ref<BookInfo[]>([])
const hostStatus = ref<HostStatus>({ running: false, port: 0, deviceId: '', nickname: '' })
const logs = ref<string[]>([])

function log(msg: string) {
  logs.value.push(msg)
  if (logs.value.length > 500) logs.value.shift()
}

let offLog: (() => void) | undefined

onMounted(() => {
  init()
  offLog = EventsOn('log', (msg: string) => log(msg))
})

onUnmounted(() => {
  if (offLog) offLog()
})

async function loadSession() {
  session.value = await GetSession()
}

async function init() {
  await loadSession()
  if (session.value.loggedIn) {
    view.value = 'main'
    if (session.value.booksDir) {
      booksDir.value = session.value.booksDir
      await refreshBooks()
    }
    hostStatus.value = await GetHostStatus()
  }
}

async function chooseDir() {
  const dir = await ChooseBooksDir()
  if (dir) {
    booksDir.value = dir
    await refreshBooks()
  }
}

async function onDirChange() {
  if (booksDir.value) {
    await SetBooksDir(booksDir.value)
    await refreshBooks()
  }
}

async function refreshBooks() {
  try {
    books.value = await ScanBooks(booksDir.value)
  } catch (e: any) {
    log('扫描失败: ' + (e?.message || e))
  }
}

async function startHost() {
  try {
    const st = await StartHost()
    hostStatus.value = st
    log(`同步服务已启动，端口 ${st.port}，设备名: ${st.nickname}`)
  } catch (e: any) {
    log('启动失败: ' + (e?.message || e))
  }
}

async function stopHost() {
  await StopHost()
  hostStatus.value = await GetHostStatus()
  log('同步服务已停止')
}

async function doLogout() {
  await Logout()
  view.value = 'login'
  email.value = ''
  code.value = ''
}

</script>

<template>
  <!-- 登录页 -->
  <div v-if="view === 'login'" class="login-wrap">
    <div class="login-card">
      <div class="brand">
        <div class="brand-mark">阅</div>
        <div class="brand-title">阅 微</div>
        <div class="brand-sub">把书送进你的阅读世界</div>
      </div>
      <div class="field">
        <label>邮箱</label>
        <input v-model="email" type="email" placeholder="you@example.com" @keyup.enter="doLogin" />
      </div>
      <div class="field">
        <label>验证码</label>
        <div class="code-row">
          <input v-model="code" placeholder="6 位验证码" @keyup.enter="doLogin" />
          <button class="btn ghost" :disabled="loginBusy" @click="sendCode">发送验证码</button>
        </div>
      </div>
      <button class="btn primary wide" :disabled="loginBusy" @click="doLogin">登 录</button>
      <p class="msg">{{ loginMsg }}</p>
    </div>
  </div>

  <!-- 主界面 -->
  <div v-else class="main">
    <!-- 顶栏 -->
    <header class="topbar">
      <div class="app-title">阅微 PC 同步</div>
      <div class="status">
        <span class="dot" :class="hostStatus.running ? 'on' : 'off'"></span>
        <span v-if="hostStatus.running">运行中 · 端口 {{ hostStatus.port }}</span>
        <span v-else>未启动</span>
      </div>
      <button class="btn ghost sm" @click="doLogout">退出登录</button>
    </header>

    <div class="content">
      <!-- 账号卡片 -->
      <section class="card account">
        <div class="acc-left">
          <div class="section-label">当前账号</div>
          <div class="acc-info">
            userId: {{ session.userId }} · 昵称: {{ session.nickname }}
          </div>
        </div>
        <div class="acc-right">
          <button v-if="!hostStatus.running" class="btn primary" @click="startHost">启动同步服务</button>
          <button v-else class="btn danger" @click="stopHost">停止服务</button>
        </div>
      </section>

      <!-- 书籍目录卡片 -->
      <section class="card">
        <div class="section-label">书籍目录</div>
        <div class="dir-row">
          <input v-model="booksDir" placeholder="选择存放 .epub 的目录" @change="onDirChange" @keyup.enter="onDirChange" />
          <button class="btn ghost" @click="chooseDir">选择目录</button>
          <button class="btn ghost" @click="refreshBooks">刷新</button>
        </div>
      </section>

      <!-- 两栏：书籍列表 + 日志 -->
      <div class="columns">
        <section class="card col-books">
          <div class="col-head">
            <div class="section-label">书籍列表</div>
            <span class="count">{{ books.length }} 本</span>
          </div>
          <ul class="book-list">
            <li v-for="b in books" :key="b.uuid">{{ b.title }}</li>
            <li v-if="books.length === 0" class="empty">暂无书籍</li>
          </ul>
        </section>
        <section class="card col-logs">
          <div class="section-label">运行日志</div>
          <div class="logs">
            <div v-for="(l, i) in logs" :key="i" class="log-line">{{ l }}</div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>
