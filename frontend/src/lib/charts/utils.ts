export const setZoomCallback = (chart: any, cb: (st: number, et: number) => void, st?: number, lt?: number) => {
  if (!chart || !cb) return;
  chart.off('datazoom');
  chart.on('datazoom', (e: any) => {
    if (e.batch && e.batch.length > 0) {
      const b = e.batch[0];
      if (b.startValue !== undefined && b.endValue !== undefined) {
        // startValue and endValue are epoch ms from xAxis type: 'time'
        cb(Math.floor(b.startValue * 1e6), Math.floor(b.endValue * 1e6));
      } else if (b.start === 0 && b.end === 100) {
        cb(0, 0);
      }
    } else if (e.start !== undefined && e.end !== undefined) {
      if (st && lt && lt > st) {
        const range = lt - st;
        cb(Math.floor(st + range * (e.start / 100)), Math.floor(st + range * (e.end / 100)));
      }
    }
  });
};
