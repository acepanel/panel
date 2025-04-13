import { $gettext } from '@/utils'

// 变通方法，由于 gettext 不能直接对动态标题进行翻译
export function translateTitle(key: string): string {
  const titles: { [key: string]: string } = {
    // 主菜单标题
    Apps: $gettext('Apps'),
    Backup: $gettext('Backup'),
    Certificate: $gettext('Certificate'),
    Container: $gettext('Container'),
    Dashboard: $gettext('Dashboard'),
    Update: $gettext('Update'),
    Database: $gettext('Database'),
    Files: $gettext('Files'),
    Firewall: $gettext('Firewall'),
    Monitoring: $gettext('Monitoring'),
    Settings: $gettext('Settings'),
    Terminal: $gettext('Terminal'),
    Tasks: $gettext('Tasks'),
    Website: $gettext('Website'),
    // 应用标题
    'Fail2ban Manager': $gettext('Fail2ban Manager'),
    'S3fs Manager': $gettext('S3fs Manager'),
    'Supervisor Manager': $gettext('Supervisor Manager'),
    'Rsync Manager': $gettext('Rsync Manager'),
    'Frp Manager': $gettext('Frp Manager'),
    'Rat Benchmark': $gettext('Rat Benchmark'),
    'System Toolbox': $gettext('System Toolbox')
  }

  return titles[key] || key
}
