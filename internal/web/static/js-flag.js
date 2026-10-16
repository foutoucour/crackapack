// Flags the page as scripted before it is painted, so the stylesheet can hide what a script is about
// to rearrange. It is loaded in the head, without defer, on purpose.
document.documentElement.classList.add('js');
