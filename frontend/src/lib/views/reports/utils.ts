export function ipToNum(ip: string): number {
  if (!ip) return -1;
  const parts = ip.trim().split(".");
  if (parts.length === 4) {
    let num = 0;
    for (let i = 0; i < 4; i++) {
      const octet = parseInt(parts[i], 10);
      if (isNaN(octet) || octet < 0 || octet > 255) return -1;
      num = num * 256 + octet;
    }
    return num;
  }
  return -1;
}

export function getVendor(mac: string): string {
  if (!mac) return "Unknown / Generic";
  const clean = mac.replace(/[:-]/g, "").toUpperCase();
  if (clean.startsWith("525400") || clean.startsWith("00163E") || clean.startsWith("080027")) return "QEMU / KVM / Virtual";
  if (clean.startsWith("000C29") || clean.startsWith("005056") || clean.startsWith("000569")) return "VMware";
  if (clean.startsWith("001A2B") || clean.startsWith("00000C")) return "Cisco Systems";
  if (clean.startsWith("00A0DE") || clean.startsWith("AC44F2")) return "Yamaha Network";
  if (clean.startsWith("F01898") || clean.startsWith("ACDE48")) return "Apple";
  if (clean.startsWith("B827EB") || clean.startsWith("DCA632")) return "Raspberry Pi";
  return "Network Equipment";
}

export function isVirtualMachine(d: any): boolean {
  const v = (d.vendor || "").toLowerCase();
  const n = (d.name || "").toLowerCase();
  const m = (d.mac || "").toUpperCase().replace(/[:-]/g, "");
  return (
    v.includes("vmware") ||
    v.includes("qemu") ||
    v.includes("kvm") ||
    v.includes("virtual") ||
    v.includes("virtualbox") ||
    v.includes("hyper-v") ||
    v.includes("xen") ||
    v.includes("parallels") ||
    n.includes("vmware") ||
    n.includes("qemu") ||
    n.includes("kvm") ||
    n.includes("vbox") ||
    m.startsWith("525400") ||
    m.startsWith("00163E") ||
    m.startsWith("080027") ||
    m.startsWith("000569") ||
    m.startsWith("000C29") ||
    m.startsWith("005056") ||
    m.startsWith("00155D")
  );
}

export function getServiceName(port: number, proto: string): string {
  if (port === 443) return "TLS / HTTPS (TCP 443)";
  if (port === 80 || port === 8080) return "HTTP (TCP 80/8080)";
  if (port === 53) return "DNS (UDP/TCP 53)";
  if (port === 1812 || port === 1813) return "RADIUS (UDP 1812/1813)";
  if (port === 161 || port === 162) return "SNMP (UDP 161/162)";
  if (port === 123) return "NTP (UDP 123)";
  if (port === 22) return "SSH (TCP 22)";
  if (port === 514) return "Syslog (UDP 514)";
  if (port === 389 || port === 636) return "LDAP / LDAPS";
  if (port === 445 || port === 139) return "SMB / CIFS";
  if (port === 3389) return "RDP (TCP 3389)";
  if (port === 1883 || port === 8883) return "MQTT (TCP 1883/8883)";
  return `${proto.toUpperCase()}/${port}`;
}

export interface ParsedFlow {
  time: number;
  src: string;
  srcPort: number;
  dst: string;
  dstPort: number;
  proto: string;
  bytes: number;
  packets: number;
  dur: number;
  tcpFlags?: string;
  reason?: string;
}
