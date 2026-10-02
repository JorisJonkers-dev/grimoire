<script setup lang="ts">
import { computed, ref, useAttrs, useId } from 'vue'

/** A rule returns what is wrong with a value, or nothing. */
export type FieldRule = (value: string) => string | undefined

const props = withDefaults(
  defineProps<{
    label: string
    type?: 'text' | 'number' | 'email' | 'search' | 'url'
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
    <p v-if="error" :id="`${id}-error`" role="alert" class="g-float__error">{{ error }}</p>
    <p v-else-if="hint" :id="`${id}-hint`" class="g-float__hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
.g-float {
  position: relative;
  display: flex;
  flex-direction: column;
}
input,
textarea {
  box-sizing: border-box;
  width: 100%;
  min-height: 52px;
  padding: 22px 12px 6px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  font: inherit;
  font-size: 16px;
}
textarea {
  resize: vertical;
}
label {
  position: absolute;
  top: 16px;
  left: 13px;
  color: var(--color-text-3);
  font-size: 16px;
  pointer-events: none;
  transition:
    top var(--motion-max),
    font-size var(--motion-max),
    color var(--motion-max);
}
input:focus,
textarea:focus {
  outline: none;
  border-color: var(--color-gold);
}
input:focus + label,
input:not(:placeholder-shown) + label,
textarea:focus + label,
textarea:not(:placeholder-shown) + label {
  top: 6px;
  font-size: 12px;
  color: var(--color-gold-high);
}
.g-float--wrong input,
.g-float--wrong textarea {
  border-color: var(--color-enemy);
}
.g-float__error,
.g-float__hint {
  margin: 4px 0 0 2px;
  font-size: 13px;
}
.g-float__error {
  color: var(--color-enemy-soft);
}
.g-float__hint {
  color: var(--color-text-3);
}
</style>
