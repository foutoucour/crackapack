// Keeps what depends on the energy live: it asks the server for the gauge of the header and for the
// price and wait of each set's booster while the page is open, so that the energy that recharges
// shows up without a reload, on the gauge and on the buttons that open boosters. The server says the
// prices and the waits, the script holds no rule of its own.
const REFRESH_MS = 30_000;

const gauge = document.querySelector('[data-energy]');

function showGauge(reading) {
  const meter = gauge.querySelector('[data-energy-meter]');
  meter.max = reading.reserveCap;
  meter.value = reading.reserve;
  gauge.querySelector('[data-energy-text]').textContent = `${reading.reserve} / ${reading.reserveCap}`;
  const saved = gauge.querySelector('[data-energy-saved]');
  saved.hidden = reading.savings === 0;
  saved.textContent = reading.savings ? `+${reading.savings} saved` : '';
  const next = gauge.querySelector('[data-energy-next]');
  next.hidden = reading.next === '';
  next.textContent = reading.next ? `+1 in ${reading.next}` : '';
}

// showPacks says, for each set shown on the page, what its booster costs and how long it waits. The
// button opens it when the energy pays for it or a top-up in coins does, and the form says which.
function showPacks(packs) {
  packs.forEach(({ set, wait, price, label, hint, topUp }) => {
    document.querySelectorAll(`[data-pack="${CSS.escape(set)}"]`).forEach((pack) => {
      const hintLine = pack.querySelector('[data-pack-wait]');
      if (!hintLine) return; // A set with welcome boosters left has nothing to wait for.
      hintLine.hidden = !hint;
      hintLine.textContent = hint ?? '';
      const open = pack.querySelector('[data-pack-open]');
      if (open) open.disabled = wait !== '' && !topUp;
      if (!price) return;
      const priceText = pack.querySelector('[data-pack-price]');
      if (priceText) priceText.textContent = price;
      if (open && open.dataset.label) open.setAttribute('aria-label', `${open.dataset.label}, ${label}`);
      const field = pack.querySelector('[data-pack-topup]');
      if (field) field.value = topUp ? 'yes' : '';
    });
  });
}

async function refresh() {
  try {
    const response = await fetch('/energy', { headers: { Accept: 'application/json' } });
    if (!response.ok) return;
    const reading = await response.json();
    showGauge(reading);
    showPacks(reading.packs);
  } catch {
    // The server may be restarting: the next tick tries again.
  }
}

if (gauge) {
  setInterval(() => { if (!document.hidden) refresh(); }, REFRESH_MS);
  // A tab that was in the background catches up as soon as it is seen.
  document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
}
