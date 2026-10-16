// A card of the binder opens big when it is clicked.
import { createViewer, dropBrokenPictures } from './cards.js';

const viewer = createViewer();
document.querySelectorAll('[data-binder]').forEach((binder) => {
  dropBrokenPictures(binder);
  binder.querySelectorAll('[data-card]').forEach((card) => {
    const open = card.querySelector('.view');
    open.addEventListener('click', () => viewer.show(card, open));
  });
});
