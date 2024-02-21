<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import { ref, defineComponent, h, reactive } from 'vue';
import { useMessage, NIcon, NButton, NInput } from "naive-ui";
import {
    Book20Regular as BookIcon,
    Person20Regular as PersonIcon,
    DrinkWine20Regular as WineIcon
} from "@vicons/fluent";

window.$message = useMessage();
const users = ref([]);

const createColumns = ({
showUsername
}) => {
return [
    {
        title: 'User ID',
        key: 'userId'
    },
    {
        title: 'Username',
        key: 'userName',
        render(row, index) {
            return h(NInput, {
                value: row.userName,
                onUpdateValue(v) {
                    users.value[index].userName = v;
                },
                onBlur() {
                    console.log(users.value[index].userName)
                }
            });
        }
    },
    {
        title: 'Email',
        key: 'email',
        render(row, index) {
            return h(NInput, {
                value: row.email,
                onUpdateValue(v) {
                    users.value[index].email = v;
                    console.log(v)
                }
            });
        }
    },
    {
        title: 'Create Date',
        key: 'createDate'
    },
    {
        title: 'Role',
        key: 'role'
    },
    {
        title: 'UUID',
        key: 'uuid'
    },
    {
    title: "Action",
    key: "actions",
    render(row) {
        return h(
        NButton,
        {
            strong: true,
            tertiary: true,
            size: "small",
            onClick: () => showUsername(row)
        },
        { default: () => "Play" }
        );
    }
    }
    ]
};
const pagination = {
        pageSize: 10
}
const columns = createColumns(
    {
        showUsername(row) {
            window.$message.info(`Username ${row.userName}`);
        }
    }
);

const getUserPage = () => {
        axios({
            method: 'get',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/admin/getUserPage?page=1&rowsPerPage=2",
            headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
            }
        }).then((response) => {
            if (response.data.msgCode === 200)
            {
                users.value = response.data.t;
            }else {
                window.$message.error("Get User Information Error!")
            }
        })
        .catch(function (error) { // 请求失败处理
            window.$message.error(error);
        });
}

getUserPage();
</script>

<template>
  <div>
    <n-space vertical size="large" >
    <n-layout>
      <n-layout-header id="header1">
        <img src="@/assets/logo.ico" id="img1" @click="$router.push('/thepersonincharge/dashboard')"/>
        <n-gradient-text :size="20" :gradient="{
          from: 'rgb(255 255 255)',
          to: 'rgb(245 245 245)'
        }" id="photolyText" @click="$router.push('/thepersonincharge/dashboard')">PHOTOLY ADMIN</n-gradient-text>
        <div id="naviContainer">
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="$router.push('/thepersonincharge/dashboard')">
                Dashboard
            </n-button>
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="$router.push('/thepersonincharge/usermanage')">
                User Mng
            </n-button>
            
        </div>
      </n-layout-header>
    </n-layout>
  </n-space>

  <div style="align-items: center; display: flex; align-content: center; 
    justify-content: center; margin-top: 5vh;">
    <n-space>
        <n-card title="" size="medium" style="border-radius: 10px; width: 85vw">
          <n-space vertical>
            <n-h2>User Management</n-h2>
            <n-data-table
                :columns="columns"
                :data="users"
                :pagination="pagination"
                :bordered="false"
                striped
            />
          </n-space>
          
        </n-card>
    </n-space>
    
  </div>
    
  </div>
</template>

<script>

export default defineComponent({
  data() {
    return {

    }
  },
  setup () {
  },
  methods: {
    checkIfIsAdmin () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/isAdmin",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.t)
        {
        }else {
          window.$message.error("You are not an admin!")
          $router.push('/login');
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
        $router.push('/login');
      });
    },
  },
  computed: {

  },
  mounted () {
    window.$message = useMessage();
    this.checkIfIsAdmin();
  },
  components: {

  },
  props: {
    
  }
})

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
