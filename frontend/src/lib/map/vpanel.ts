import P5 from 'p5'
import inconslata from '../../assets/fonts/inconsolata.ttf';
import port from "../../assets/images/port.png";
let ports :any = [];
let power = false;
let rotate = false;
let vpanelZoom = 1.0;
let portWrap = 16;
let _vpanelP5 :P5 | undefined  = undefined;
let cw = 1000;
let ch = 400;

const vpanelMain = (p: any) => {
  const PORT_SIZE = 150;
  const LED_SIZE = 10;
  const LED_XOFFSET = 300;
  const LED_YOFFSET = 75;
  let portImage: any;
  let font: any;
  let width: number;
  let height: number;
  let depth: number;
  let r = 0;

  p.preload = () => {
    try {
      font = p.loadFont(inconslata);
    } catch (e) {
      console.warn("Could not preload inconsolata font:", e);
    }
    try {
      portImage = p.loadImage(port);
    } catch (e) {
      console.warn("Could not preload port image:", e);
    }
  };

  p.setup = () => {
    p.createCanvas(cw, ch, p.WEBGL);
    p.frameRate(10);
    if (font) {
      try {
        p.textFont(font, 24);
      } catch (e) {
        console.warn("Could not set textFont:", e);
      }
    }
    p.camera(100, -500, 2000, 0, 0, 0);
  };

  p.draw = () => {
    const pCount = Array.isArray(ports) ? ports.length : 0;
    width = (pCount < portWrap ? pCount : portWrap) * PORT_SIZE + PORT_SIZE;
    height = Math.ceil(pCount / portWrap) * ((5 * PORT_SIZE) / 4) + PORT_SIZE / 2;
    if (pCount === 0) {
      height = (5 * PORT_SIZE) / 4;
    }
    depth = pCount < 8 ? 800 : 1000;

    // 背景色
    p.background(128);
    // カメラの制御
    p.orbitControl();
    p.scale(vpanelZoom);
    if (rotate) {
      r += 0.1;
    }
    p.rotateY(r);
    p.noStroke();
    p.textAlign(p.CENTER);

    p.push();
    p.fill(50, 50, 50);
    p.box(width, height, depth);

    for (let i = 0; i < pCount; i++) {
      const x = i % portWrap;
      const y = Math.floor(i / portWrap);
      const pt = ports[i] || {};

      p.push();
      // Port
      p.translate(
        -width / 2 + x * PORT_SIZE + PORT_SIZE,
        -height / 2 + y * ((5 * PORT_SIZE) / 4) + (3 * PORT_SIZE) / 4 + 10,
        depth / 2 + 1
      );
      if (portImage) {
        p.texture(portImage);
      } else {
        p.fill(30, 30, 30);
      }
      p.plane(150, 150);

      p.push();
      // Link up LED
      p.translate(2 * LED_SIZE - PORT_SIZE / 2, 2 * LED_SIZE - PORT_SIZE / 2, 0);
      p.fill(pt.State === "up" ? "#11ee00" : "#999");
      p.sphere(LED_SIZE, 8, 8);
      p.pop();

      // Speed LED
      p.push();
      p.translate(
        -2 * LED_SIZE + PORT_SIZE / 2,
        2 * LED_SIZE - PORT_SIZE / 2,
        0
      );
      p.fill(
        pt.Speed > 0 && pt.Speed < 1000 * 1000 * 1000
          ? "#eeaa00"
          : "#999"
      );
      p.sphere(LED_SIZE, 8, 8);
      p.pop();

      if (font) {
        p.push();
        p.fill("#ccc");
        p.text(i + 1 + "", 0, (3 * PORT_SIZE) / 4 - 10);
        p.pop();
      }
      p.pop();

      // 裏面
      p.push();
      p.translate(
        width / 2 - i * LED_SIZE * 3 - LED_XOFFSET,
        -height / 2 + LED_YOFFSET,
        -depth / 2 - 1
      );
      p.push();
      // LED
      p.fill(pt.State === "up" ? "#11ee00" : "#999");
      p.sphere(LED_SIZE, 8, 8);
      p.pop();

      if (font) {
        p.push();
        p.fill("#ccc");
        p.rotateY(p.radians(180.0));
        p.text(i + 1 + "", 0, 60);
        p.pop();
      }
      p.pop();
    }

    p.push();
    p.translate(width / 2 - 100, -height / 2 + 50, -depth / 2 - 1);
    p.push();
    // LED
    p.fill(power ? "#2211ff" : "#999");
    p.sphere(LED_SIZE, 8, 8);
    p.pop();

    if (font) {
      p.push();
      p.fill("#ccc");
      p.rotateY(p.radians(180.0));
      p.text("POWER", 0, 60);
      p.pop();
    }
    p.pop();

    p.pop();
  };
};

export const setVPanel = (po: any, pw: any, r: any, z: number, pwv: number) => {
  ports = po;
  power = pw;
  rotate = r;
  vpanelZoom = z || 1.0;
  portWrap = pwv || 16;
};

export const initVPanel = (div: string) => {
  const d = document.getElementById(div);
  if (!d) {
    return;
  }
  cw = Math.max(d.clientWidth || 0, 600);
  ch = Math.max(d.clientHeight || 0, 350);
  if (_vpanelP5) {
    try {
      _vpanelP5.remove();
    } catch (e) {
      console.error("Error removing old vpanel in initVPanel:", e);
    }
    _vpanelP5 = undefined;
  }
  try {
    _vpanelP5 = new P5(vpanelMain, d);
  } catch (e) {
    console.error("Error creating P5 vpanel:", e);
  }
};

export const deleteVPanel = () => {
  if (_vpanelP5) {
    try {
      _vpanelP5.remove();
    } catch (e) {
      console.error("Error removing vpanel:", e);
    }
    _vpanelP5 = undefined;
  }
};