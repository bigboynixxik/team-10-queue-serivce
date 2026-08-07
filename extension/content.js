const TARGET_CONTAINER_SELECTOR = "._64647206970c5686";
const BUTTON_ID = "avito-queue-service-button";
const QUEUE_SERVICE_URL = "http://localhost:3000/avito";

function addQueueButton() {
  const container = document.querySelector(TARGET_CONTAINER_SELECTOR);

  if (!container || container.querySelector(`#${BUTTON_ID}`)) return;

  const button = document.createElement("button");
  button.id = BUTTON_ID;
  button.type = "button";
  button.textContent = "Перейти в очередь";
  button.addEventListener("click", () => {
    window.location.assign(QUEUE_SERVICE_URL);
  });

  container.prepend(button);
}

addQueueButton();

const observer = new MutationObserver(addQueueButton);
observer.observe(document.documentElement, { childList: true, subtree: true });
