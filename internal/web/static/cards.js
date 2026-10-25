// What the booster page and the collection page do with a card, as a module they both import.

// describe says what a card is, for a screen reader: its name, its rarity and its tags.
export function describe(card) {
  const parts = [card.querySelector('.name').textContent, card.dataset.rarity];
  card.querySelectorAll('.tag').forEach((tag) => parts.push(tag.textContent));
  return parts.join(', ');
}

// dropBrokenPictures makes a card whose picture cannot load fall back to the card's text, which is
// always in the page, inside container.
export function dropBrokenPictures(container) {
  const drop = (img) => {
    // An empty slot's picture has no card to fall back to: it just goes.
    img.closest('[data-card]')?.classList.remove('has-image');
    img.remove();
  };
  // An error event does not bubble, so listen while it travels down.
  container.addEventListener('error', (e) => {
    if (e.target.tagName === 'IMG') drop(e.target);
  }, true);
  container.querySelectorAll('img').forEach((img) => {
    if (img.complete && img.naturalWidth === 0) drop(img);
  });
}

// createViewer adds a dialog that shows one card big, and returns it. show(card, from) opens a copy
// of card, and when the dialog closes, focus goes back to from. Options: tools is an element whose
// copy goes under the card, with its buttons and forms working, and label names the dialog when
// describe(card) cannot, as for an empty slot.
export function createViewer() {
  const dialog = document.createElement('dialog');
  dialog.className = 'viewer';
  const closeButton = document.createElement('button');
  closeButton.type = 'button';
  closeButton.className = 'primary close';
  closeButton.textContent = 'Close';

  let openedFrom = null;
  // A click closes the dialog, but not one on the tools, which are there to be used.
  dialog.addEventListener('click', (e) => {
    if (!e.target.closest('.tools')) dialog.close();
  });
  dialog.addEventListener('close', () => {
    dialog.replaceChildren();
    // Not every browser focuses a button on click, so say where focus goes back to.
    if (openedFrom) openedFrom.focus();
  });
  document.body.append(dialog);

  return {
    show(card, from, { tools, label } = {}) {
      openedFrom = from;
      const copy = card.cloneNode(true);
      // The buttons of the card are for the page, not for the dialog.
      copy.querySelectorAll('.view, .pick, .tools').forEach((control) => control.remove());
      dialog.setAttribute('aria-label', label ?? describe(card));
      dialog.replaceChildren(copy);
      if (tools) dialog.append(tools.cloneNode(true));
      dialog.append(closeButton);
      dialog.showModal();
      closeButton.focus();
    },
  };
}
