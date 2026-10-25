// The collection page: a card opens big with its tools, and the list of binders.
import { createViewer, dropBrokenPictures } from './cards.js';

// A click on a slot opens its card big, with what the player can do with it. An empty slot has no
// card to show, so its dialog is named after the card it stands for.
const viewer = createViewer();
document.querySelectorAll('[data-binder]').forEach((binder) => {
  dropBrokenPictures(binder);
  binder.querySelectorAll('.entry').forEach((entry) => {
    const pick = entry.querySelector('.pick');
    const tools = entry.querySelector('.tools');
    const face = entry.querySelector('[data-card], .slot');
    const label = face.matches('[data-card]') ? undefined : `${tools.querySelector('.tools-name').textContent}, not in your collection`;
    pick.addEventListener('click', () => viewer.show(face, pick, { tools, label }));
  });
});

// A link to a card, such as a quest's reward, ends in the anchor of its slot, so that the page scrolls
// there, and asks for focus=<card>: the card opens big, as if its slot was clicked.
const focused = new URLSearchParams(location.search).get('focus');
if (focused) document.getElementById(`c-${focused}`)?.querySelector('.pick')?.click();

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
