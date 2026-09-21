import * as echarts from 'echarts';

export const stateList = [
  { text: '重度障害', color: '#e31a1c', icon: 'mdi-alert-circle', value: 'high' },
  { text: '軽度障害', color: '#fb9a99', icon: 'mdi-alert-circle', value: 'low' },
  { text: '注意', color: '#dfdf22', icon: 'mdi-alert', value: 'warn' },
  { text: '正常', color: '#33a02c', icon: 'mdi-check-circle', value: 'normal' },
  { text: '復旧', color: '#1f78b4', icon: 'mdi-autorenew', value: 'repair' },
  { text: '情報', color: '#1f78b4', icon: 'mdi-information', value: 'info' },
  { text: '停止', color: '#777', icon: 'mdi-stop', value: 'off' },
  { text: 'Down', color: '#e31a1c', icon: 'mdi-alert-circle', value: 'down' },
  { text: 'Up', color: '#33a02c', icon: 'mdi-check-circle', value: 'up' },
  { text: '不明', color: '#999', icon: 'mdi-comment-question-outline', value: 'unknown' },
];

export const stateMap: Record<string, any> = {};
stateList.forEach((e) => {
  stateMap[e.value] = e;
});

export const getStateColor = (state: string): string => {
  return stateMap[state] ? stateMap[state].color : '#999';
};

export const getStateName = (state: string): string => {
  return stateMap[state] ? stateMap[state].text : '不明';
};

export const addrModeList = [
  { name: '固定 IP アドレス', value: 'ip' },
  { name: '固定 MAC アドレス', value: 'mac' },
  { name: 'ホスト名', value: 'host' },
];

export const snmpModeList = [
  { name: 'SNMPv2c', value: 'v2c' },
  { name: 'SNMPv3 (AuthNoPriv)', value: 'v3auth' },
  { name: 'SNMPv3 (AuthPriv)', value: 'v3authpriv' },
  { name: 'SNMPv1', value: 'v1' },
];

export const iconList = [
  { name: 'デスクトップ', icon: 'mdi-monitor', value: 'desktop', code: 0xf0379 },
  { name: 'デスクトップ (Classic)', icon: 'mdi-desktop-classic', value: 'desktop-classic', code: 0xf07c0 },
  { name: 'ノートPC', icon: 'mdi-laptop', value: 'laptop', code: 0xf0322 },
  { name: 'タブレット', icon: 'mdi-tablet', value: 'tablet', code: 0xf04f6 },
  { name: 'サーバー', icon: 'mdi-server', value: 'server', code: 0xf048b },
  { name: 'ネットワーク機器', icon: 'mdi-ip-network', value: 'hdd', code: 0xf0a60 },
  { name: 'IPデバイス', icon: 'mdi-ip-network', value: 'ip', code: 0xf0a60 },
  { name: 'ネットワーク', icon: 'mdi-lan', value: 'network', code: 0xf0317 },
  { name: 'Wi-Fi', icon: 'mdi-wifi', value: 'wifi', code: 0xf05a9 },
  { name: 'クラウド', icon: 'mdi-cloud', value: 'cloud', code: 0xf015f },
  { name: 'プリンター', icon: 'mdi-printer', value: 'printer', code: 0xf042a },
  { name: 'スマホ / 携帯', icon: 'mdi-cellphone', value: 'cellphone', code: 0xf011c },
  { name: 'ルーター', icon: 'mdi-router', value: 'router', code: 0xf11e2 },
  { name: 'Webサーバー', icon: 'mdi-web', value: 'web', code: 0xf059f },
  { name: 'データベース', icon: 'mdi-database', value: 'db', code: 0xf01bc },
  { name: 'Wi-Fi AP', icon: 'mdi-router-wireless', value: 'mdi-router-wireless', code: 0xf0469 },
  { name: 'スイッチ', icon: 'mdi-switch', value: 'switch', code: 0xf04e4 },
  { name: 'NAS', icon: 'mdi-nas', value: 'nas', code: 0xf08f3 },
  { name: '監視カメラ', icon: 'mdi-cctv', value: 'camera', code: 0xf07ae },
  { name: 'UPS', icon: 'mdi-battery-charging', value: 'ups', code: 0xf0084 },
  { name: 'セキュリティ', icon: 'mdi-security', value: 'security', code: 0xf0483 },
  { name: 'Windows', icon: 'mdi-microsoft-windows', value: 'windows', code: 0xf05b3 },
  { name: 'Linux', icon: 'mdi-linux', value: 'linux', code: 0xf033d },
  { name: 'Raspberry Pi', icon: 'mdi-raspberry-pi', value: 'raspberrypi', code: 0xf043f },
  { name: 'IoT / ボード', icon: 'mdi-developer-board', value: 'iot', code: 0xf0697 },
];

const iconCodeMap = new Map<string, string>();
const iconMap = new Map<string, string>();

iconList.forEach((e) => {
  iconMap.set(e.value, e.icon);
  iconCodeMap.set(e.value, String.fromCodePoint(e.code));
});

export const getIcon = (icon: string): string => {
  return iconMap.get(icon) || 'mdi-comment-question-outline';
};

export const getIconCode = (icon: string): string => {
  return iconCodeMap.get(icon) || String.fromCodePoint(0xf0379);
};

export const formatTime = (date: any, format = '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}') => {
  return echarts.time.format(date, format, false);
};

export const renderTime = (t: number) => {
  if (t < 1) return '';
  const d = new Date(t / (1000 * 1000));
  return formatTime(d);
};
