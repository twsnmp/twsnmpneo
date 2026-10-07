import * as echarts from 'echarts';
import { get } from 'svelte/store';
import { locale } from 'svelte-i18n';

export const stateList = [
  { text: '重度障害', textEn: 'High Severity', color: '#e31a1c', icon: 'mdi-alert-circle', value: 'high' },
  { text: '軽度障害', textEn: 'Low Severity', color: '#fb9a99', icon: 'mdi-alert-circle', value: 'low' },
  { text: '注意', textEn: 'Warning', color: '#dfdf22', icon: 'mdi-alert', value: 'warn' },
  { text: '正常', textEn: 'Normal', color: '#33a02c', icon: 'mdi-check-circle', value: 'normal' },
  { text: '復帰', textEn: 'Repaired', color: '#1f78b4', icon: 'mdi-autorenew', value: 'repair' },
  { text: '情報', textEn: 'Info', color: '#1f78b4', icon: 'mdi-information', value: 'info' },
  { text: '新規', textEn: 'New', color: '#1f78b4', icon: 'mdi-information', value: 'New' },
  { text: '変更', textEn: 'Change', color: '#e31a1c', icon: 'mdi-autorenew', value: 'Change' },
  { text: '停止', textEn: 'Disabled', color: '#777', icon: 'mdi-stop', value: 'off' },
  { text: 'Down', textEn: 'Down', color: '#e31a1c', icon: 'mdi-alert-circle', value: 'down' },
  { text: 'Up', textEn: 'Up', color: '#33a02c', icon: 'mdi-check-circle', value: 'up' },
  { text: '不明', textEn: 'Unknown', color: '#999', icon: 'mdi-comment-question-outline', value: 'unknown' },
];

export const stateMap: Record<string, any> = {};
stateList.forEach((e) => {
  stateMap[e.value] = e;
  stateMap[e.value.toLowerCase()] = e;
});

export const getStateColor = (state: string): string => {
  if (!state) return '#999';
  return stateMap[state] ? stateMap[state].color : (stateMap[state.toLowerCase()] ? stateMap[state.toLowerCase()].color : '#999');
};

export const getStateName = (state: string, t?: (key: string) => string): string => {
  if (t) {
    const translated = t(`status.${state}`);
    if (translated && !translated.startsWith('status.')) {
      return translated;
    }
  }
  const isJa = (get(locale) || 'ja').startsWith('ja');
  const entry = stateMap[state] || (state ? stateMap[state.toLowerCase()] : null);
  if (!entry) return isJa ? '不明' : 'Unknown';
  return isJa ? entry.text : (entry.textEn || entry.text);
};

export const addrModeList = [
  { name: 'IP固定', nameEn: 'Fixed IP', value: 'ip' },
  { name: 'MAC固定', nameEn: 'Fixed MAC', value: 'mac' },
  { name: 'ホスト名固定', nameEn: 'Fixed Host', value: 'host' },
];

export const getAddrModeName = (val: string): string => {
  const isJa = (get(locale) || 'ja').startsWith('ja');
  const entry = addrModeList.find(e => e.value === val);
  if (!entry) return val;
  return isJa ? entry.name : (entry.nameEn || entry.name);
};

export const snmpModeList = [
  { name: 'SNMPv2c', value: 'v2c' },
  { name: 'SNMPv3 (AuthNoPriv)', value: 'v3auth' },
  { name: 'SNMPv3 (AuthPriv)', value: 'v3authpriv' },
  { name: 'SNMPv1', value: 'v1' },
];

export interface IconEnt {
  name: string;
  nameEn?: string;
  icon?: string;
  value: string;
  code?: number;
  type?: 'mdi' | 'image';
  image?: string;
}

export const defaultIconList: IconEnt[] = [
  { name: 'デスクトップ', nameEn: 'Desktop', icon: 'mdi-monitor', value: 'desktop', code: 0xf0379, type: 'mdi' },
  { name: 'デスクトップ (Classic)', nameEn: 'Desktop (Classic)', icon: 'mdi-desktop-classic', value: 'desktop-classic', code: 0xf07c0, type: 'mdi' },
  { name: 'ノートPC', nameEn: 'Laptop', icon: 'mdi-laptop', value: 'laptop', code: 0xf0322, type: 'mdi' },
  { name: 'タブレット', nameEn: 'Tablet', icon: 'mdi-tablet', value: 'tablet', code: 0xf04f6, type: 'mdi' },
  { name: 'サーバー', nameEn: 'Server', icon: 'mdi-server', value: 'server', code: 0xf048b, type: 'mdi' },
  { name: 'ネットワーク機器', nameEn: 'Network Device', icon: 'mdi-ip-network', value: 'hdd', code: 0xf0a60, type: 'mdi' },
  { name: 'IPデバイス', nameEn: 'IP Device', icon: 'mdi-ip-network', value: 'ip', code: 0xf0a60, type: 'mdi' },
  { name: 'ネットワーク', nameEn: 'Network', icon: 'mdi-lan', value: 'network', code: 0xf0317, type: 'mdi' },
  { name: 'Wi-Fi', nameEn: 'Wi-Fi', icon: 'mdi-wifi', value: 'wifi', code: 0xf05a9, type: 'mdi' },
  { name: 'クラウド', nameEn: 'Cloud', icon: 'mdi-cloud', value: 'cloud', code: 0xf015f, type: 'mdi' },
  { name: 'プリンター', nameEn: 'Printer', icon: 'mdi-printer', value: 'printer', code: 0xf042a, type: 'mdi' },
  { name: 'スマホ / 携帯', nameEn: 'Smartphone / Mobile', icon: 'mdi-cellphone', value: 'cellphone', code: 0xf011c, type: 'mdi' },
  { name: 'ルーター', nameEn: 'Router', icon: 'mdi-router', value: 'router', code: 0xf11e2, type: 'mdi' },
  { name: 'Webサーバー', nameEn: 'Web Server', icon: 'mdi-web', value: 'web', code: 0xf059f, type: 'mdi' },
  { name: 'データベース', nameEn: 'Database', icon: 'mdi-database', value: 'db', code: 0xf01bc, type: 'mdi' },
  { name: 'Wi-Fi AP', nameEn: 'Wi-Fi AP', icon: 'mdi-router-wireless', value: 'mdi-router-wireless', code: 0xf0469, type: 'mdi' },
  { name: 'スイッチ', nameEn: 'Switch', icon: 'mdi-switch', value: 'switch', code: 0xf04e4, type: 'mdi' },
  { name: 'NAS', nameEn: 'NAS', icon: 'mdi-nas', value: 'nas', code: 0xf08f3, type: 'mdi' },
  { name: '監視カメラ', nameEn: 'Surveillance Camera', icon: 'mdi-cctv', value: 'camera', code: 0xf07ae, type: 'mdi' },
  { name: 'UPS', nameEn: 'UPS', icon: 'mdi-battery-charging', value: 'ups', code: 0xf0084, type: 'mdi' },
  { name: 'セキュリティ', nameEn: 'Security', icon: 'mdi-security', value: 'security', code: 0xf0483, type: 'mdi' },
  { name: 'Windows', nameEn: 'Windows', icon: 'mdi-microsoft-windows', value: 'windows', code: 0xf05b3, type: 'mdi' },
  { name: 'Linux', nameEn: 'Linux', icon: 'mdi-linux', value: 'linux', code: 0xf033d, type: 'mdi' },
  { name: 'Raspberry Pi', nameEn: 'Raspberry Pi', icon: 'mdi-raspberry-pi', value: 'raspberrypi', code: 0xf043f, type: 'mdi' },
  { name: 'IoT / ボード', nameEn: 'IoT / Dev Board', icon: 'mdi-developer-board', value: 'iot', code: 0xf0697, type: 'mdi' },
];

export const customIconList: IconEnt[] = [];
export const iconList: IconEnt[] = [...defaultIconList];

const iconCodeMap = new Map<string, string>();
const iconMap = new Map<string, string>();
const iconImageMap = new Map<string, string>();

function rebuildIconMaps() {
  iconMap.clear();
  iconCodeMap.clear();
  iconImageMap.clear();
  iconList.length = 0;
  defaultIconList.forEach((e) => {
    iconList.push(e);
    if (e.icon) iconMap.set(e.value, e.icon);
    if (e.code) {
      iconCodeMap.set(e.value, String.fromCodePoint(e.code));
      iconCodeMap.set(e.name, String.fromCodePoint(e.code));
    }
  });
  customIconList.forEach((e) => {
    iconList.push(e);
    if (e.icon) iconMap.set(e.value, e.icon);
    if (e.type === 'image' && e.image) {
      iconImageMap.set(e.value, e.image);
      iconImageMap.set(e.name, e.image);
    } else if (e.code) {
      iconCodeMap.set(e.value, String.fromCodePoint(e.code));
      iconCodeMap.set(e.name, String.fromCodePoint(e.code));
    }
  });
}
rebuildIconMaps();

export function setCustomIcons(icons: Array<{
  name?: string;
  Name?: string;
  code?: number;
  Code?: number;
  value?: string;
  icon?: string;
  nameEn?: string;
  type?: 'mdi' | 'image';
  Type?: 'mdi' | 'image';
  image?: string;
  Image?: string;
}>) {
  customIconList.length = 0;
  icons.forEach((ic) => {
    const name = ic.name || ic.Name || '';
    if (!name) return;
    const type = (ic.type || ic.Type || (ic.image || ic.Image ? 'image' : 'mdi')) as 'mdi' | 'image';
    const image = ic.image || ic.Image || '';
    const code = Number(ic.code ?? ic.Code ?? 0);
    if (type !== 'image' && !code) return;
    const value = ic.value || name;
    customIconList.push({
      name,
      nameEn: ic.nameEn || name,
      value,
      code: type === 'image' ? 0 : code,
      type,
      image,
      icon: ic.icon || '',
    });
  });
  rebuildIconMaps();
}

export const getImageIconList = (): IconEnt[] => {
  return customIconList.filter(e => e.type === 'image');
};

export const getIconName = (val: string): string => {
  const isJa = (get(locale) || 'ja').startsWith('ja');
  const entry = iconList.find(e => e.value === val || e.name === val);
  if (!entry) return val;
  return isJa ? entry.name : (entry.nameEn || entry.name);
};

export const isImageIcon = (icon: string): boolean => {
  if (!icon) return false;
  if (icon.startsWith('data:image/')) return true;
  if (iconImageMap.has(icon)) return true;
  const entry = customIconList.find(e => e.value === icon || e.name === icon);
  return Boolean(entry && entry.type === 'image' && entry.image);
};

export const getIconImage = (icon: string): string => {
  if (!icon) return '';
  if (icon.startsWith('data:image/')) return icon;
  if (iconImageMap.has(icon)) return iconImageMap.get(icon)!;
  const entry = customIconList.find(e => e.value === icon || e.name === icon);
  if (entry && entry.image) {
    iconImageMap.set(icon, entry.image);
    return entry.image;
  }
  return '';
};

export const getIcon = (icon: string): string => {
  if (!icon) return 'mdi-comment-question-outline';
  return iconMap.get(icon) || 'mdi-comment-question-outline';
};

export const getIconCode = (icon: string): string => {
  if (!icon) return String.fromCodePoint(0xf0379);
  if (iconCodeMap.has(icon)) return iconCodeMap.get(icon)!;
  const num = parseInt(icon, icon.startsWith('0x') || icon.startsWith('0X') ? 16 : 10);
  if (!isNaN(num) && num > 0) {
    try {
      return String.fromCodePoint(num);
    } catch {
      // fallback
    }
  }
  return iconCodeMap.get('desktop') || String.fromCodePoint(0xf0379);
};


export const formatTime = (date: any, format = '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}') => {
  try {
    return echarts.time.format(date, format, false);
  } catch {
    const d = date instanceof Date ? date : new Date(date);
    return isNaN(d.getTime()) ? '-' : d.toLocaleString();
  }
};

export const formatTimeStr = (t: number | string | Date | undefined | null, format = '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}'): string => {
  if (t === undefined || t === null || t === '') return '-';
  if (t instanceof Date) {
    return isNaN(t.getTime()) ? '-' : formatTime(t, format);
  }
  let ms: number;
  if (typeof t === 'string') {
    const num = Number(t);
    if (!isNaN(num) && num > 0) {
      t = num;
    } else {
      const d = new Date(t);
      return isNaN(d.getTime()) ? String(t) : formatTime(d, format);
    }
  }
  if (typeof t === 'number') {
    if (t <= 0) return '-';
    if (t > 1e16) {
      // Nanoseconds (e.g., 1.7e18) -> divide by 1,000,000 to get ms
      ms = Math.floor(t / 1e6);
    } else if (t > 1e13) {
      // Microseconds (e.g., 1.7e15) -> divide by 1,000 to get ms
      ms = Math.floor(t / 1e3);
    } else if (t > 1e10) {
      // Milliseconds (e.g., 1.7e12)
      ms = t;
    } else {
      // Seconds (e.g., 1.7e9) -> multiply by 1,000 to get ms
      ms = t * 1000;
    }
    const d = new Date(ms);
    return isNaN(d.getTime()) ? '-' : formatTime(d, format);
  }
  return '-';
};

export const renderTime = (t: number) => {
  if (!t || t < 1) return '';
  return formatTimeStr(t);
};

export const renderDuration = (sec: number): string => {
  const isJa = (get(locale) || 'ja').startsWith('ja');
  if (!sec || sec <= 0) return isJa ? '0秒' : '0s';
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = Math.floor(sec % 60);
  const parts: string[] = [];
  if (isJa) {
    if (d > 0) parts.push(`${d}日`);
    if (h > 0) parts.push(`${h}時間`);
    if (m > 0) parts.push(`${m}分`);
    if (s > 0 || parts.length === 0) parts.push(`${s}秒`);
  } else {
    if (d > 0) parts.push(`${d}d`);
    if (h > 0) parts.push(`${h}h`);
    if (m > 0) parts.push(`${m}m`);
    if (s > 0 || parts.length === 0) parts.push(`${s}s`);
  }
  return parts.join(' ');
};

export const renderBytes = (bytes: number): string => {
  if (!bytes || bytes <= 0) return '0.000B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const clampedI = Math.max(0, Math.min(i, sizes.length - 1));
  const val = (bytes / Math.pow(k, clampedI)).toFixed(3);
  return `${val}${sizes[clampedI]}`;
};

export const renderSLA = (sla: number): string => {
  if (typeof sla !== 'number' || isNaN(sla)) return '100.000%';
  const val = Math.max(0, Math.min(100, sla));
  return val.toFixed(3) + '%';
};

export const renderTimeMili = (t: number | string | Date | undefined | null): string => {
  if (t === undefined || t === null || t === '') return '-';
  let ms: number = 0;
  if (typeof t === 'number') {
    if (t <= 0) return '-';
    if (t > 1e16) ms = Math.floor(t / 1e6);
    else if (t > 1e13) ms = Math.floor(t / 1e3);
    else if (t > 1e10) ms = t;
    else ms = t * 1000;
  } else if (typeof t === 'string') {
    const num = Number(t);
    if (!isNaN(num) && num > 0) {
      return renderTimeMili(num);
    }
    const d = new Date(t);
    if (!isNaN(d.getTime())) ms = d.getTime();
    else return t;
  } else if (t instanceof Date) {
    if (!isNaN(t.getTime())) ms = t.getTime();
    else return '-';
  }
  const d = new Date(ms);
  return formatTime(d, '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}.{SSS}');
};

export const severityNames = [
  'emerg',
  'alert',
  'crit',
  'err',
  'warning',
  'notice',
  'info',
  'debug',
];

export const facilityNames = [
  'kern',
  'user',
  'mail',
  'daemon',
  'auth',
  'syslog',
  'lpr',
  'news',
  'uucp',
  'cron',
  'authpriv',
  'ftp',
  'ntp',
  'logaudit',
  'logalert',
  'clock',
  'local0',
  'local1',
  'local2',
  'local3',
  'local4',
  'local5',
  'local6',
  'local7',
];

export const getSyslogType = (sv: number, fac: number): string => {
  const sName = sv >= 0 && sv < severityNames.length ? severityNames[sv] : 'unknown';
  const fName = fac >= 0 && fac < facilityNames.length ? facilityNames[fac] : 'unknown';
  return `${sName}:${fName}`;
};

export const renderSpeed = (bps: number): string => {
  if (!bps || bps <= 0 || isNaN(bps)) return '0 bps';
  const units = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps'];
  let val = bps;
  let idx = 0;
  while (val >= 1000 && idx < units.length - 1) {
    val /= 1000;
    idx++;
  }
  return `${val.toFixed(2)} ${units[idx]}`;
};

export const renderPercent = (v: number | undefined | null): string => {
  if (v === undefined || v === null || isNaN(v)) return '0.00%';
  return v.toFixed(2) + '%';
};

export const typeList = [
  { name: 'PING', value: 'ping' },
  { name: 'SNMP', value: 'snmp' },
  { name: 'gNMI', value: 'gnmi' },
  { name: 'TCP', value: 'tcp' },
  { name: 'HTTP', value: 'http' },
  { name: 'TLS', value: 'tls' },
  { name: 'DNS', value: 'dns' },
  { name: 'NTP', value: 'ntp' },
  { name: 'SYSLOG', value: 'syslog' },
  { name: 'SNMP TRAP', value: 'trap' },
  { name: 'ARP Log', value: 'arplog' },
  { name: 'NetFlow', value: 'netflow' },
  { name: 'Command', value: 'cmd' },
  { name: 'SSH', value: 'ssh' },
  { name: 'Report', value: 'report' },
  { name: 'TWSNMP', value: 'twsnmp' },
  { name: 'TwLogEye', value: 'twlogeye' },
  { name: 'Pi-Hole', value: 'pihole' },
  { name: 'LXI', value: 'lxi' },
  { name: 'Monitor', value: 'monitor' },
  { name: 'MQTT', value: 'mqtt' },
  { name: 'EMAIL', value: 'email' },
  { name: 'STUN', value: 'stun' },
];

const pollingTypeMap = new Map<string, string>();
typeList.forEach((e) => {
  pollingTypeMap.set(e.value, e.name);
});

export const renderPollingType = (type: string): string => {
  return pollingTypeMap.get(type) || type.toUpperCase() || 'Unknown';
};

export const getScoreColor = (s: number): string => {
  if (s < 34) {
    return getStateColor('repair');
  } else if (s <= 50) {
    return getStateColor('info');
  } else if (s < 68) {
    return getStateColor('warn');
  } else if (s < 77) {
    return getStateColor('low');
  }
  return getStateColor('high');
};

export const getScoreIcon = (s: number): string => {
  if (s < 34) {
    return 'mdi-emoticon-excited-outline';
  } else if (s <= 50) {
    return 'mdi-emoticon-outline';
  } else if (s < 68) {
    return 'mdi-emoticon-sad-outline';
  } else if (s < 77) {
    return 'mdi-emoticon-sick-outline';
  }
  return 'mdi-emoticon-dead-outline';
};

export const logModeList = [
  { value: 0, text: '記録しない', textEn: 'None', key: 'none' },
  { value: 1, text: '毎回記録', textEn: 'Always', key: 'always' },
  { value: 2, text: '状態変化時', textEn: 'On Change', key: 'onChange' },
  { value: 3, text: '異常検知あり', textEn: 'AI Anomaly', key: 'ai' },
];

export const getLogModeName = (val: number | undefined | null, t?: (key: string) => string): string => {
  const mode = Number(val || 0);
  const entry = logModeList.find((e) => e.value === mode) || logModeList[0];
  if (t) {
    const translated = t(`polling.logModes.${entry.key}`);
    if (translated && !translated.startsWith('polling.logModes.')) {
      return translated;
    }
  }
  const isJa = (get(locale) || 'ja').startsWith('ja');
  return isJa ? entry.text : entry.textEn;
};

export const getLogModeBadgeClass = (val: number | undefined | null): string => {
  const mode = Number(val || 0);
  switch (mode) {
    case 1: // always
      return 'bg-sky-50 dark:bg-sky-500/10 text-sky-700 dark:text-sky-300 border-sky-200 dark:border-sky-500/30';
    case 2: // onChange
      return 'bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-300 border-amber-200 dark:border-amber-500/30';
    case 3: // ai
      return 'bg-purple-50 dark:bg-purple-500/10 text-purple-700 dark:text-purple-300 border-purple-200 dark:border-purple-500/30';
    case 0: // none
    default:
      return 'bg-slate-100 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 border-slate-200 dark:border-slate-700';
  }
};
