<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import NavigationBar from "../components/NavigationBar.vue"
import HomePart from "../components/HomePart.vue"
import GalleryPart from "../components/GalleryPart.vue"
import TagPart from "../components/TagPart.vue"
import SearchPart from "../components/SearchPart.vue"
import ManagePart from "../components/ManagePart.vue"
import { shallowRef } from 'vue'


</script>

<template>
  <div>
    <n-alert title="Warning" type="error" style="border-radius: 15px; margin-bottom: 1rem" v-show="warningShow">
          {{ errors }}
    </n-alert>
    <NavigationBar :avatarAddr="avatarAddr"/>
    <component :is="componentParts[part]" style="padding: 1.5rem; margin-left: 1.5rem;"/>
  </div>
</template>

<script>

export default{
  data() {
    return {
      loading: false,
      warningShow: false,
      errors: "",
      userInfo: {},
      componentParts: {},
      avatarAddr: "@/assets/logo.ico"
    }
  },
  props: {
    part: String
  },
  setup () {
    return {

    }
      
  },
  methods: {
    showWarning(error) {
      this.warningShow = true
      this.errors = error
      setTimeout(() => {this.warningShow = false}, 5000);
    },
  },
  computed: {

  },
  mounted () {
    if (localStorage.getItem("HRD-Token") === null)
    {
      router.push("/login");
    }else {
      this.avatarAddr = import.meta.env.VITE_APP_BASE_URL + "/user/getAvatar/" + localStorage.getItem("HRD-Token")

      this.componentParts["HomePart"] = shallowRef(HomePart)
      this.componentParts["GalleryPart"] = shallowRef(GalleryPart)
      this.componentParts["TagPart"] = shallowRef(TagPart)
      this.componentParts["SearchPart"] = shallowRef(SearchPart)
      this.componentParts["ManagePart"] = shallowRef(ManagePart)
    }
  },
  components: {

  }
}

</script>

<style scoped>

</style>
