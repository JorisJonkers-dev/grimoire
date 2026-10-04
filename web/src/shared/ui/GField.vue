<script setup lang="ts">
import { computed, ref, useAttrs, useId } from 'vue'

/** A rule returns what is wrong with a value, or nothing. */
export type FieldRule = (value: string) => string | undefined

const props = withDefaults(
  defineProps<{
    label: string
    type?: 'text' | 'number' | 'email' | 'search' | 'url' | 'password'
    required?: boolean
    rules?: FieldRule[]
    hint?: string
    multiline?: boolean
    maxlength?: number
    autocomplete?: string
  }>(),
  { type: 'text', required: false, rules: () => [], hint: '', multiline: false, maxlength: undefined, autocomplete: 'off' },
)
// Attributes such as data-testid or inputmode land on the control itself; class and style stay on the wrapper.
defineOptions({ inheritAttrs: false })
const attrs = useAttrs()
const wrapperStyle = computed(() => attrs.style as string | undefined)
const control = computed(() => Object.fromEntries(Object.entries(attrs).filter(([k]) => k !== 'class' && k !== 'style')))
const model = defineModel<string | number>({ default: '' })
const id = useId()
// A value is judged once the field is left, never while it is being typed.
const left = ref(false)

const error = computed(() => {
  if (!left.value) return ''
  const value = String(model.value).trim()
  if (props.required && value === '') return `${props.label} is required.`
  for (const rule of props.rules) {
    const wrong = rule(value)
    if (wrong) return wrong
  }
  return ''
})

// Once left, a field that holds something right says so.
const right = computed(() => left.value && error.value === '' && String(model.value).trim() !== '' && (props.required || props.rules.length > 0))

function input(ev: Event) {
  const raw = (ev.target as HTMLInputElement | HTMLTextAreaElement).value
  model.value = props.type === 'number' && raw !== '' ? Number(raw) : raw
}

defineExpose({
  /** Judges the field now, as if it had been left; true when it is fine. */
  validate(): boolean {
    left.value = true
    return error.value === ''
  },
})
</script>

<template>
  <div :class="['g-float', attrs.class, { 'g-float--wrong': error }]" :style="wrapperStyle">
    <textarea
      v-if="multiline"
      v-bind="control"
      :id="id"
      :value="model"
      placeholder=" "
      rows="3"
      :maxlength="maxlength"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="error ? `${id}-error` : hint ? `${id}-hint` : undefined"
      @input="input"
      @blur="left = true"
    />
    <input
      v-else
      v-bind="control"
      :id="id"
      :value="model"
      :type="type"
      placeholder=" "
      :maxlength="maxlength"
      :autocomplete="autocomplete"
      :required="required"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="error ? `${id}-error` : hint ? `${id}-hint` : undefined"
      @input="input"
      @blur="left = true"
    />
    <label :for="id">{{ label }}</label>
    <span v-if="right" class="g-float__right" aria-hidden="true" data-testid="field-right">✓</span>
    <p v-if="error" :id="`${id}-error`" role="alert" class="g-float__error">{{ error }}</p>
    <p v-else-if="hint" :id="`${id}-hint`" class="g-float__hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
/* Filled, with a rule under it: the label sits inside and floats up once there is something to read. */
.g-float {
  position: relative;
  display: flex;
  flex-direction: column;
}
input,
textarea {
  box-sizing: border-box;
  width: 100%;
  min-height: var(--size-field);
  padding: 22px 14px 6px;
  border: 0;
  border-radius: var(--radius-control) var(--radius-control) 0 0;
  background: var(--color-field);
  box-shadow: inset 0 -1px 0 var(--color-line);
  color: var(--color-text);
  font: inherit;
  font-size: 16px;
}
textarea {
  min-height: 92px;
  padding-top: 26px;
  line-height: 1.55;
  resize: vertical;
}
label {
  position: absolute;
  top: 18px;
  left: 14px;
  font-family: var(--font-label);
  font-size: 16px;
  color: var(--color-text-2);
  pointer-events: none;
  transition:
    top 120ms ease-out,
    font-size 120ms ease-out,
    color 120ms ease-out;
}
input:focus,
textarea:focus {
  outline: none;
  box-shadow: inset 0 -2px 0 var(--color-brass-edge);
}
input:focus + label,
input:not(:placeholder-shown) + label,
textarea:focus + label,
textarea:not(:placeholder-shown) + label {
  top: 7px;
  font-size: 12px;
}
input:focus + label,
textarea:focus + label {
  color: var(--color-gold-high);
}
.g-float--wrong input,
.g-float--wrong textarea {
  box-shadow: inset 0 -2px 0 var(--color-danger-edge);
}
.g-float__right {
  position: absolute;
  top: 19px;
  right: 14px;
  color: var(--color-success);
}
.g-float__error,
.g-float__hint {
  margin: 5px 0 0;
  padding-left: 14px;
  font-size: 13px;
}
.g-float__error {
  color: var(--color-danger-text);
}
.g-float__hint {
  color: var(--color-text-3);
}
</style>
