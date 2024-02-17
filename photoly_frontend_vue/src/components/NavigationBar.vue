<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
</script>

<template>
  <div>
    <n-space vertical size="large" >
    <n-layout>
      <n-layout-header id="header1">
        <img src="@/assets/logo.ico" id="img1" v-on:click="changeHomeComponent('HomePart')"/>
        <n-gradient-text :size="20" :gradient="{
          from: 'rgb(255 255 255)',
          to: 'rgb(245 245 245)'
        }" id="photolyText" v-on:click="changeHomeComponent('HomePart')">PHOTOLY</n-gradient-text>
        <div id="naviContainer">
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="changeHomeComponent('HomePart')">
                Home
            </n-button>
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="changeHomeComponent('GalleryPart')">
                Gallery
            </n-button>
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="changeHomeComponent('TagPart')">
                Tag
            </n-button>
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="changeHomeComponent('SearchPart')">
                Search
            </n-button>
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="logOut">
                Logout
            </n-button>
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="changeHomeComponent('ManagePart')">
                Manage
            </n-button>
            <img :src="avatarAddr" id="userImg" v-on:click="changeHomeComponent('ManagePart')"/>
            
        </div>
      </n-layout-header>
    </n-layout>
  </n-space>
  </div>
</template>

<script>

export default{
  data() {
    return {
      warningShow: false,
      errors: "",
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
    changeHomeComponent(cp_name) {
        // this.$emit('updateCurrComponent', cp_name);
        router.push("/home/"+cp_name);
    },
    logOut() {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/user/signOut/",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          localStorage.removeItem("HRD-Token")
          router.push("/");
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
  computed: {

  },
  mounted () {

  },
  components: {

  },
  props: {
    avatarAddr: String
  }
}

</script>

<style scoped>
#header1 {
  font-size: 20px;
  background-color: #38b2ac;
  height: 4rem;
}
#img1 {
  width: 50px;
  border-radius: 30px;
  vertical-align: middle;
  padding-left: 0.4rem;
  padding-top: 0.4rem;
  padding-bottom: 0.4rem;
  margin-left: 10px;
  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}
#photolyText {
  vertical-align: middle;
  margin-left: 10px;
  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}
#naviContainer {
  float:right;
  padding-top: 0.5rem;
  margin-right: 20px;
  
}
.naviBarBtn {
  color: aliceblue;
  font-size: 1rem;
  margin-top: 0.4rem;
}
.naviBarBtn:hover {
  color: #EDF2F7;
}
#userImg {
  width: 45px;
  border-radius: 30px;
  vertical-align:top;


  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
  
}
.n-menu .n-menu-item-content .n-menu-item-content-header {
  color: white;
}
</style>
