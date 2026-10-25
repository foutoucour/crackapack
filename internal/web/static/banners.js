// Every banner that tells the player of something can be closed. Without this script a banner just
// stays until the page changes.
document.querySelectorAll('.results').forEach((banner) => {
  const close = document.createElement('button');
  close.type = 'button';
  close.className = 'banner-close';
  close.setAttribute('aria-label', 'Close this message');
  close.textContent = '\u00d7';
  close.addEventListener('click', () => banner.remove());
  banner.append(close);
});
