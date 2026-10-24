// The collection page: the tools of a card, the list of binders, and the card big.
import { createViewer, dropBrokenPictures } from './cards.js';

// The View button of a card, among its tools, opens it big.
const viewer = createViewer();
document.querySelectorAll('[data-binder]').forEach((binder) => {
  dropBrokenPictures(binder);
  binder.querySelectorAll('[data-card] .tool-view').forEach((open) => {
    open.addEventListener('click', () => viewer.show(open.closest('[data-card]'), open));
  });
});

// A click on a slot shows its tools and the name of its card, one slot at a time. A click anywhere
// else, or Escape, puts them away.
const picks = [...document.querySelectorAll('.binder .pick')];
const isOpen = (pick) => pick.getAttribute('aria-expanded') === 'true';
const setOpen = (pick, open) => {
  pick.setAttribute('aria-expanded', String(open));
  const entry = pick.closest('.entry');
  entry.classList.toggle('selected', open);
  if (!open) entry.querySelectorAll('details[open]').forEach((panel) => { panel.open = false; });
};
const closeAll = (except) => picks.forEach((pick) => { if (pick !== except) setOpen(pick, false); });
picks.forEach((pick) => pick.addEventListener('click', () => {
  closeAll(pick);
  setOpen(pick, !isOpen(pick));
}));
document.addEventListener('click', (e) => {
  if (!e.target.closest('.entry')) closeAll();
});
document.addEventListener('keydown', (e) => {
  const open = picks.find(isOpen);
  if (e.key !== 'Escape' || !open) return;
  closeAll();
  open.focus();
});

// On a wide screen the list of binders stays open beside the binder, and on a narrow one it folds
// into the name of the set in view. Without this script it stays open.
const picker = document.querySelector('.set-picker');
const wide = window.matchMedia('(min-width: 56rem)');
const syncPicker = () => { picker.open = wide.matches; };
wide.addEventListener('change', syncPicker);
syncPicker();

// Typing in the search box keeps the sets whose name holds what was typed.
const search = document.querySelector('[data-set-search]');
const entries = [...document.querySelectorAll('[data-set-entry]')];
search.addEventListener('input', () => {
  const typed = search.value.trim().toLowerCase();
  entries.forEach((entry) => {
    entry.hidden = !entry.querySelector('.set-name').textContent.toLowerCase().includes(typed);
  });
});
