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

  const shelf = document.createElement('ol');
  shelf.className = 'shelf';
  shelf.setAttribute('aria-label', 'Cards already revealed');
  booster.after(shelf);

  // The controls are one bar over the card, wearing the icon of the set like the page's own bar, the
  // way out once the booster is shown, which waits, hidden, under the booster until the reveal is over.
  const actions = document.querySelector('.actions');
  const controls = document.createElement('div');
  controls.className = 'controls';
  const progress = document.createElement('strong');
  progress.className = 'bar-progress';
  const skip = makeButton('Skip to the end', 'bar-link');
  skip.setAttribute('aria-keyshortcuts', 'Shift+Space');
  skip.title = 'Shift+Space';
  const advance = makeButton('', 'primary');
  advance.setAttribute('aria-keyshortcuts', 'Space');
  advance.title = 'Space';
  const info = document.createElement('div');
  info.className = 'bar-info';
  info.append(progress, skip);
  const icon = actions.querySelector('.bar-icon');
  if (icon) controls.append(icon.cloneNode(true));
  controls.append(info, advance);
  booster.before(controls);
  actions.hidden = true;
  // The sets the booster unlocked are told at the end: that would take room from the card, and give
  // the cards away.
  const unlocked = document.querySelector('.unlock');
  if (unlocked) unlocked.hidden = true;

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
  skip.addEventListener('click', skipToEnd);

  // Space turns the card and brings the next one up, Shift+Space skips to the end. Another button, a
  // link or a field keeps its own use of the key, and so does an open dialog. The advance button is
  // the exception: Space would press it, which is what the key does anyway.
  const keepsSpace = (target) => target !== advance && target.closest('a, button, input, select, textarea, [contenteditable]');
  const isRevealKey = (e) => e.code === 'Space' && controls.isConnected && !keepsSpace(e.target) && !document.querySelector('dialog[open]');
  let spaceHandled = false;
  let finishedBySpace = false;
  document.addEventListener('keydown', (e) => {
    if (!isRevealKey(e)) return;
    e.preventDefault();
    spaceHandled = true;
    if (e.repeat) return;
    finishedBySpace = true;
    if (e.shiftKey) skipToEnd();
    else step();
    finishedBySpace = false;
  });
  // A button is pressed when its space key comes up. The last card hands the focus to the next
  // booster's button, which must not take the key that turned that card.
  document.addEventListener('keyup', (e) => {
    if (e.code !== 'Space' || !spaceHandled) return;
    e.preventDefault();
    spaceHandled = false;
  });

  function skipToEnd() {
    for (; index < lastIndex; index += 1) {
      cards[index].classList.remove('facedown');
      toShelf(cards[index]);
    }
    cards[lastIndex].classList.remove('facedown');
    present(lastIndex);
    finish('All cards revealed.');
  }

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
    progress.textContent = `Card ${i + 1} of ${cards.length}`;
    if (i === lastIndex) {
      status.textContent = 'One card left: the rare.';
      advance.textContent = 'Reveal the rare';
    } else {
      status.textContent = `Card ${i + 1} of ${cards.length}, face down.`;
      advance.textContent = 'Reveal';
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

  // finish swaps the controls for the bar of what comes next and hands it the keyboard focus, the
  // next booster first, without scrolling away from the card that was just turned. When the space
  // key finished the reveal the focus goes to the way out instead: a player pressing the key on
  // through the cards must not open, and pay for, another booster with one press too many.
  function finish(message) {
    status.textContent = message;
    controls.remove();
    actions.hidden = false;
    if (unlocked) unlocked.hidden = false;
    const next = finishedBySpace
      ? actions.querySelector('a')
      : actions.querySelector('button:not(:disabled)') || actions.querySelector('a');
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
