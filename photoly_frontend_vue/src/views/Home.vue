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
import { ref, shallowRef } from 'vue'

</script>

<template>
  <div>
    <n-alert title="Warning" type="error" style="border-radius: 15px; margin-bottom: 1rem" v-show="warningShow">
          {{ errors }}
    </n-alert>
    <NavigationBar @updateCurrComponent="updateCurrComponent"/>
    <component :is="componentParts[currComponent]" style="padding: 1.5rem;"/>
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
      currComponent: "HomePart",
      componentParts: {
        HomePart, GalleryPart, TagPart, SearchPart, ManagePart
      }
    }
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
    updateCurrComponent(cp_name) {
      this.currComponent = cp_name;
    }
  },
  computed: {

  },
  mounted () {
    if (localStorage.getItem("HRD-Token") === null)
    {
      router.push("/login");
    }else {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/user/getInfo/",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          let userT = response.data.t
          this.userInfo = userT
        }else {
          this.showWarning("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        this.showWarning(error);
      });
    }
  },
  components: {

  }
}

</script>

<style scoped>

</style>
