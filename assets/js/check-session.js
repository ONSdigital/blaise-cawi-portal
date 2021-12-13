window.addEventListener('click', function(event) {
  event = event || window.event;
  var target = event.target || event.srcElement;
  if (
    target.getAttribute("role") == "button" ||
    target.tagName == "button" ||
    target.parentElement.getAttribute("role") == "button" ||
    target.parentElement.tagName == "button"
  ) {
    var xmlHttp = new XMLHttpRequest
    xmlHttp.open("GET", "/auth/logged-in", false);
    xmlHttp.send(null);
    if (xmlHttp.status !== 200) {
      this.window.location.replace("/auth/timed-out");
    };
  }
}, false);
