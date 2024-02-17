<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
</script>

<template>
  <div>
    ManagePart
  </div>
</template>

<script>

export default{
  data() {
    return {
      
    }
  },
  setup () {
    return {
      
    }
  },
  methods: {
    
  },
  computed: {

  },
  mounted () {
    if (true)
    {

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
