<script lang="ts">
  import aiCatImg from '../../assets/images/aicat_thumb.jpg';

  let { size = "w-6 h-6", class: className = "", rounded = "rounded-full" } = $props<{
    size?: string;
    class?: string;
    rounded?: string;
  }>();
</script>

<!-- TWSNMP NEO AI Cat Detective Character Avatar (Ultra-fast optimized thumbnail) -->
<div class="relative inline-flex items-center justify-center shrink-0 overflow-hidden {rounded} border border-cyan-500/30 bg-gradient-to-br from-cyan-500/10 to-indigo-500/20 shadow-sm transition-transform hover:scale-105 {size} {className}">
  <img
    src={aiCatImg}
    alt="AI Cat Assistant"
    class="w-full h-full object-cover {rounded}"
    loading="eager"
    decoding="async"
    width="80"
    height="80"
  />
</div>
