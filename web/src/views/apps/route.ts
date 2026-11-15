import type { RoutesType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

const views = import.meta.glob('./*/IndexView.vue')

const titles: Record<string, string> = {
  apache: 'Apache',
  caddy: 'Caddy',
  clickhouse: 'ClickHouse',
  codeserver: 'Code Server',
  docker: 'Docker',
  elasticsearch: 'ElasticSearch',
  fail2ban: 'Fail2ban Manager',
  frp: 'Frp Manager',
  gitea: 'Gitea',
  grafana: 'Grafana',
  kafka: 'Kafka',
  mariadb: 'MariaDB',
  memcached: 'Memcached',
  minio: 'MinIO',
  mongodb: 'MongoDB',
  mysql: 'MySQL',
  nginx: 'Nginx',
  openlitespeed: 'OpenLiteSpeed',
  openresty: 'OpenResty',
  opensearch: 'OpenSearch',
  percona: 'Percona',
  pgadmin: 'pgAdmin',
  phpmyadmin: 'phpMyAdmin',
  podman: 'Podman',
  postgresql: 'PostgreSQL',
  prometheus: 'Prometheus',
  pureftpd: 'Pure-FTPd',
  redis: 'Redis',
  rocketmq: 'RocketMQ',
  rsync: 'Rsync Manager',
  s3fs: 'S3fs Manager',
  stalwart: 'Stalwart',
  supervisor: 'Supervisor Manager',
  valkey: 'Valkey',
}

export default Object.entries(titles).map(([slug, title]) => ({
  name: slug,
  path: `/apps/${slug}`,
  component: Layout,
  isHidden: true,
  children: [
    {
      name: `apps-${slug}-index`,
      path: '',
      component: views[`./${slug}/IndexView.vue`],
      meta: {
        title,
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
})) as RoutesType
