// Turns a booster page into a reveal: one big card at a time, face down until the player turns it,
// and the rare, which is always last, held back for the end. Cards already seen drop to a shelf
// below. Without this script the page shows every card face up, so nothing depends on it.
import { createViewer, describe, dropBrokenPictures } from './cards.js';

(() => {
  const booster = document.querySelector('[data-booster]');
  if (!booster) return;

  const status = document.getElementById('reveal-status');
  const cards = Array.from(booster.querySelectorAll('[data-card]'));
  const lastIndex = cards.length - 1;
  const isFaceDown = (card) => card.classList.contains('facedown');
  const label = describe;

  dropBrokenPictures(booster);

  const controls = document.createElement('div');
  controls.className = 'controls';
  const advance = makeButton('', 'primary');
  const skip = makeButton('Skip to the end');
  controls.append(advance, skip);
  booster.before(controls);

  const shelf = document.createElement('ol');
  shelf.className = 'shelf';
  shelf.setAttribute('aria-label', 'Cards already revealed');
  booster.after(shelf);

  // A card on the shelf opens big, so the miniatures can be looked at again.
  const viewer = createViewer();

  booster.classList.add('staged');
  cards.forEach((card) => card.classList.add('facedown'));
  let index = 0;
  present(0);
  advance.focus();

  advance.addEventListener('click', step);
  cards.forEach((card) => card.addEventListener('click', () => {
    if (card.classList.contains('current')) step();
  }));
  skip.addEventListener('click', () => {
    for (; index < lastIndex; index += 1) {
      cards[index].classList.remove('facedown');
      toShelf(cards[index]);
    }
    cards[lastIndex].classList.remove('facedown');
    present(lastIndex);
    finish('All cards revealed.');
  });

  // step turns the current card, or, once it is turned, brings the next one up.
  function step() {
    const card = cards[index];
    if (isFaceDown(card)) {
      card.classList.remove('facedown');
      card.classList.add('just-flipped');
      if (index === lastIndex) {
        finish(`${label(card)}. That was the last card.`);
        return;
      }
      status.textContent = `${label(card)}. Card ${index + 1} of ${cards.length}.`;
      advance.textContent = 'Next card';
      return;
    }
    toShelf(card);
    index += 1;
    present(index);
  }

  // present makes cards[i] the big face-down card and says what the button will do.
  function present(i) {
    const card = cards[i];
    card.classList.add('current');
    card.classList.toggle('last', i === lastIndex);
    if (i === lastIndex) {
      status.textContent = 'One card left: the rare.';
      advance.textContent = 'Reveal the rare';
    } else {
      status.textContent = `Card ${i + 1} of ${cards.length}, face down.`;
      advance.textContent = `Reveal card ${i + 1} of ${cards.length}`;
    }
  }

  function toShelf(card) {
    card.classList.remove('current', 'just-flipped', 'last');
    const open = makeButton('', 'view');
    open.setAttribute('aria-label', `View ${card.querySelector('.name').textContent}`);
    open.addEventListener('click', () => viewer.show(card, open));
    card.append(open);
    shelf.append(card);
  }

  // finish drops the controls and hands the keyboard focus to what comes next, without scrolling
  // away from the card that was just turned.
  function finish(message) {
    status.textContent = message;
    controls.remove();
    const next = document.querySelector('.actions button:not(:disabled), .actions a');
    if (next) next.focus({ preventScroll: true });
  }

  function makeButton(text, className) {
    const button = document.createElement('button');
    button.type = 'button';
    button.textContent = text;
    if (className) button.className = className;
    return button;
  }
})();
