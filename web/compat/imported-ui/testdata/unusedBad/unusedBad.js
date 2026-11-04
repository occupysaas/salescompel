import { LightningElement, api } from 'lwc';
export default class AccountCard extends LightningElement {
  @api recordId = '';
  @api name = '';
  draft = '';
  connectedCallback() { this.draft = this.name; }
  handleInput(event) { this.draft = event.target.value; }
  handleSave() { this.dispatchEvent(new CustomEvent('save', { detail: this.draft })); }
unused() { fetch('PRIVATE_MARKER'); } }
